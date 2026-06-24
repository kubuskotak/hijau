// Package service holds the domain logic shared by the REST handlers, the MCP
// server, and background jobs. The translation write path is the spine: it keeps
// the entity mutation, per-string history, project activity, and the
// base-language OUTDATED cascade atomic within a single transaction.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portierglobal/hijau/apps/api/internal/db"
	"github.com/portierglobal/hijau/apps/api/internal/i18n"
	"github.com/portierglobal/hijau/apps/api/internal/id"
	"github.com/portierglobal/hijau/apps/api/internal/store"
)

// Action is the kind of write being performed on a translation.
type Action string

const (
	SetText Action = "set_text" // create/update the translation text
	Approve Action = "approve"  // reviewer: -> reviewed
	Reject  Action = "reject"   // reviewer: -> needs_work
)

var (
	// ErrInvalid indicates a client error (bad ICU, empty approval, etc.).
	ErrInvalid = errors.New("invalid translation")
	// ErrConflict indicates an optimistic-concurrency version mismatch.
	ErrConflict = errors.New("version conflict")
)

// Actor identifies who is performing the write, for history/activity attribution.
type Actor struct {
	Kind     db.AuthorKind
	UserID   string
	APIKeyID string
}

func sysActor() Actor { return Actor{Kind: db.AuthorKindSystem} }

// SetTranslationInput is the resolved input for a single translation write. The
// caller (handler) is responsible for authorization and for loading the key and
// language.
type SetTranslationInput struct {
	Key            db.TranslationKey
	Language       db.Language
	BaseLanguageID string
	Text           string
	Action         Action
	Actor          Actor
	ExpectedVersion *int32 // optional optimistic-concurrency precondition
	// Origin overrides the stored origin on a SetText write (e.g. machine_mt,
	// machine_tm, import); empty means a human edit. is_machine is derived from
	// it (true for the machine_* origins).
	Origin db.TranslationOrigin
}

func isMachineOrigin(o db.TranslationOrigin) bool {
	return o == db.TranslationOriginMachineMt || o == db.TranslationOriginMachineTm
}

// SetTranslationResult reports the new translation and how many sibling
// translations were marked OUTDATED (only when editing the base language).
type SetTranslationResult struct {
	Translation   db.Translation
	OutdatedCount int
}

// SetTranslation applies a translation write atomically.
func SetTranslation(ctx context.Context, st *store.Store, in SetTranslationInput) (SetTranslationResult, error) {
	var res SetTranslationResult
	err := st.WithTx(ctx, func(q *db.Queries) error {
		cur, err := q.GetTranslationForUpdate(ctx, db.GetTranslationForUpdateParams{
			KeyID: in.Key.ID, LanguageID: in.Language.ID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			cur, err = q.CreateTranslation(ctx, db.CreateTranslationParams{
				ID: id.New(), KeyID: in.Key.ID, LanguageID: in.Language.ID,
				State: db.TranslationStateUntranslated, Origin: db.TranslationOriginHuman,
			})
		}
		if err != nil {
			return err
		}
		if in.ExpectedVersion != nil && cur.Version != *in.ExpectedVersion {
			return ErrConflict
		}

		newText, newState, newOrigin := cur.Text, cur.State, cur.Origin
		switch in.Action {
		case SetText:
			if err := validateText(ctx, q, in); err != nil {
				return err
			}
			newText = pgTextOrNull(in.Text)
			newOrigin = db.TranslationOriginHuman
			if in.Origin != "" {
				newOrigin = in.Origin
			}
			if strings.TrimSpace(in.Text) == "" {
				newState = db.TranslationStateUntranslated
			} else {
				newState = db.TranslationStateTranslated
			}
		case Approve:
			if !cur.Text.Valid || strings.TrimSpace(cur.Text.String) == "" {
				return fmt.Errorf("%w: cannot approve an empty translation", ErrInvalid)
			}
			newState = db.TranslationStateReviewed
		case Reject:
			newState = db.TranslationStateNeedsWork
		default:
			return fmt.Errorf("%w: unknown action %q", ErrInvalid, in.Action)
		}

		updated, err := q.UpdateTranslation(ctx, db.UpdateTranslationParams{
			ID: cur.ID, Text: newText, State: newState, Origin: newOrigin,
			IsMachine: isMachineOrigin(newOrigin), UpdatedBy: pgTextPtr(in.Actor.UserID),
		})
		if err != nil {
			return err
		}
		if err := writeHistory(ctx, q, cur, updated, in.Actor); err != nil {
			return err
		}

		// Editing the base-language source string invalidates siblings.
		if in.Action == SetText && in.Language.ID == in.BaseLanguageID {
			newHash := i18n.SourceHash(in.Text)
			if !in.Key.SourceHash.Valid || in.Key.SourceHash.String != newHash {
				if err := q.SetKeySourceHash(ctx, db.SetKeySourceHashParams{
					ID: in.Key.ID, SourceHash: pgtype.Text{String: newHash, Valid: true},
				}); err != nil {
					return err
				}
				sibs, err := q.MarkSiblingsOutdated(ctx, db.MarkSiblingsOutdatedParams{
					KeyID: in.Key.ID, LanguageID: in.Language.ID,
				})
				if err != nil {
					return err
				}
				for _, sib := range sibs {
					if err := writeOutdatedHistory(ctx, q, sib); err != nil {
						return err
					}
				}
				res.OutdatedCount = len(sibs)
				if err := writeActivity(ctx, q, in.Key.ProjectID, db.ActivityTypeSourceChanged,
					in.Actor, in.Key.ID, "", in.Language.ID); err != nil {
					return err
				}
			}
		}

		at := db.ActivityTypeTranslationUpdated
		if in.Action != SetText {
			at = db.ActivityTypeTranslationStateChanged
		}
		if err := writeActivity(ctx, q, in.Key.ProjectID, at, in.Actor,
			in.Key.ID, updated.ID, in.Language.ID); err != nil {
			return err
		}

		res.Translation = updated
		return nil
	})
	return res, err
}

// RecordApprovedTM stores an approved translation as a translation-memory
// segment. It is best-effort and meant to run AFTER the approval commits — a
// failure here must never roll back the approval, so callers ignore the error
// (after logging). Base-language approvals and empty sources are skipped.
func RecordApprovedTM(ctx context.Context, st *store.Store, key db.TranslationKey, targetLang db.Language, baseLanguageID, targetText string) error {
	if targetLang.ID == baseLanguageID || strings.TrimSpace(targetText) == "" {
		return nil
	}
	bt, err := st.GetTranslation(ctx, db.GetTranslationParams{KeyID: key.ID, LanguageID: baseLanguageID})
	if err != nil || !bt.Text.Valid || strings.TrimSpace(bt.Text.String) == "" {
		return nil // no source text to key the memory on
	}
	baseLang, err := st.GetLanguage(ctx, baseLanguageID)
	if err != nil {
		return err
	}
	return st.InsertTMSegment(ctx, db.InsertTMSegmentParams{
		ID: id.New(), ProjectID: key.ProjectID,
		SourceLang: baseLang.Tag, TargetLang: targetLang.Tag,
		SourceText: bt.Text.String, TargetText: targetText,
		SourceHash: i18n.SourceHash(bt.Text.String),
		KeyID:      pgtype.Text{String: key.ID, Valid: true},
		Origin:     "human",
	})
}

// validateText checks the new text's ICU placeholders against the base string.
func validateText(ctx context.Context, q *db.Queries, in SetTranslationInput) error {
	base := in.Text
	if in.Language.ID != in.BaseLanguageID {
		base = ""
		if bt, err := q.GetTranslation(ctx, db.GetTranslationParams{
			KeyID: in.Key.ID, LanguageID: in.BaseLanguageID,
		}); err == nil && bt.Text.Valid {
			base = bt.Text.String
		}
	}
	if strings.TrimSpace(base) == "" {
		if !i18n.BracesBalanced(in.Text) {
			return fmt.Errorf("%w: unbalanced braces", ErrInvalid)
		}
		return nil
	}
	if err := i18n.ValidateTranslation(in.Text, base); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return nil
}

func writeHistory(ctx context.Context, q *db.Queries, old, updated db.Translation, a Actor) error {
	return q.InsertTranslationHistory(ctx, db.InsertTranslationHistoryParams{
		ID: id.New(), TranslationID: updated.ID, KeyID: updated.KeyID, LanguageID: updated.LanguageID,
		OldText: old.Text, NewText: updated.Text,
		OldState: nullState(old.State), NewState: nullState(updated.State),
		Origin: updated.Origin, AuthorKind: a.Kind,
		AuthorID: pgTextPtr(a.UserID), ApiKeyID: pgTextPtr(a.APIKeyID),
	})
}

func writeOutdatedHistory(ctx context.Context, q *db.Queries, sib db.Translation) error {
	return q.InsertTranslationHistory(ctx, db.InsertTranslationHistoryParams{
		ID: id.New(), TranslationID: sib.ID, KeyID: sib.KeyID, LanguageID: sib.LanguageID,
		OldText: sib.Text, NewText: sib.Text,
		NewState: nullState(db.TranslationStateOutdated),
		Origin:   sib.Origin, AuthorKind: db.AuthorKindSystem,
	})
}

func writeActivity(ctx context.Context, q *db.Queries, projectID string, t db.ActivityType, a Actor, keyID, translationID, languageID string) error {
	return q.InsertActivity(ctx, db.InsertActivityParams{
		ID: id.New(), ProjectID: projectID, Type: t,
		ActorID: pgTextPtr(a.UserID), ActorKind: a.Kind,
		KeyID: pgTextPtr(keyID), TranslationID: pgTextPtr(translationID), LanguageID: pgTextPtr(languageID),
	})
}

func nullState(s db.TranslationState) db.NullTranslationState {
	return db.NullTranslationState{TranslationState: s, Valid: true}
}

func pgTextOrNull(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
func pgTextPtr(s string) pgtype.Text    { return pgtype.Text{String: s, Valid: s != ""} }
