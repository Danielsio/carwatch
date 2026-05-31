---
title: "fix: Bot audit — concurrency bugs, error handling, UX, and web-bot sync"
type: fix
status: active
date: 2026-05-31
deepened: 2026-05-31
review: 4/5 agents approved (3 approve-with-nits, 1 request-changes → addressed)
---

# Bot Audit: Fix All 17 Issues (#1124–#1140)

## Overview

Comprehensive fix plan for 17 issues found during a full audit of the Telegram bot, notification system, and web-bot sync. Work is organized in 4 phases: critical bugs first, then error handling/UX polish, then notification UX, then web-bot sync. Each phase can ship independently.

## Problem Frame

The bot has concurrency bugs (race conditions in wizard state), silent error handling (users see nothing when things fail), UX gaps (no typing indicators, no action buttons on notifications, no confirmation on destructive actions), a data model issue (Hebrew gearbox values in DB), and no data sync between linked Telegram and web accounts.

## Requirements Trace

- R1. Fix concurrency bugs that can corrupt wizard state (#1124)
- R2. Surface errors to users instead of swallowing silently (#1125, #1128, #1133, #1139)
- R3. Close security gap in rate limiting (#1126)
- R4. Add confirmation to destructive "clear hidden" action (#1127)
- R5. Normalize gearbox storage to language-neutral codes (#1129)
- R6. Add typing indicators for responsive feel (#1130)
- R7. Add save/hide buttons to notification messages (#1131)
- R8. Cap notification batch size (#1132)
- R9. Warn users before wizard timeout (#1134)
- R10. Ensure web-only users always receive notifications (#1137)
- R11. Sync searches between linked Telegram/web accounts (#1135)
- R12. Sync bookmarks and settings between linked accounts (#1136)
- R13. Make quick-start configurable (#1138)
- R14. Show change diff in /edit confirmation (#1140)

## Scope Boundaries

- No new notification channels (email, SMS)
- No changes to the scraper, enricher, or scheduler pipeline
- No frontend (React) changes — web-bot sync is backend-only; web UI will adapt via existing API
- Gearbox migration (#1129) is data-compatible — old Hebrew values continue to work during transition

### Deferred to Separate Tasks

- Creating `docs/solutions/` knowledge base: after this batch completes
- Web dashboard UI changes for sync visibility: separate frontend PR

## Context & Research

### Relevant Code and Patterns

- **Bot handler pattern:** `lockChat(chatID)` + `defer unlock()` → load state → business logic → save state → send message. All within the lock.
- **Test pattern:** `newTestBot(t)` + `tb.simulateCommand/Callback/Text` + assert on `tb.msg.last()`
- **Locale pattern:** Add keys to `internal/locale/commands.go` or `wizard.go`, use `locale.T(lang, "key")`
- **Callback registration:** Exact match in `callbackExact` map, prefix match in `callbackPrefixes` slice in `callback_registry.go`
- **Storage mock pattern:** Hand-written structs implementing storage interfaces, one method per line
- **Error message pattern:** `b.send(ctx, chatID, locale.T(lang, "error_key"))` for user-facing, `b.logger.Error(...)` for internal

### External References

- Not needed — all issues are internal patterns with strong existing examples

## Key Technical Decisions

- **Phase ordering:** Bugs → Error handling/UX polish → Notification UX → Web-bot sync. Each phase is independently shippable.
- **Gearbox migration (#1129):** Store the Yad2 scraper's actual values (`"Automatic"` / `"Manual"`) as canonical codes — NOT abbreviated `"auto"/"manual"` which would break `EqualFold` filtering against listing data. Add a normalization/mapping layer that also recognizes old Hebrew values (`"אוטומט"`, `"ידני"`). No DB migration needed.
- **Web-bot sync (#1135, #1136):** Resolve linked accounts to the Telegram user's `chat_id` at query time via the existing `linked_web_id` column. No data duplication. `resolveCanonicalChatID` must fall back to the web user's own chatID when the linked Telegram user is deleted or inactive. Unlink is a destructive operation requiring user confirmation — data stays with the Telegram user.
- **Notification action buttons (#1131):** Add a `TelegramKeyboardNotifier` interface with `NotifyRawWithKeyboard`. `MultiNotifier` type-asserts to it when keyboard is present. The shared `Notifier` interface stays unchanged — no variadic options pollution.
- **Batch size limit (#1132):** Cap at 10 listings enforced upstream in the scheduler/delivery layer before publishing to Redis, not just in the formatter. Overflow gets a footer: "View N more in /history."
- **saveWizardState error handling (#1125):** Introduce a `saveWizardStateOrAbort(ctx, chatID, state, wd) bool` helper that wraps the save, sends error message on failure, and returns false. Callers use `if !b.saveWizardStateOrAbort(...) { return }` — one-line guard instead of 35 copies of 3-line error checks.

## Open Questions

### Resolved During Planning

- **Should gearbox migration backfill old rows?** No — normalization layer handles old values. Natural updates will migrate organically.
- **Should sync be bidirectional?** No — Telegram `chat_id` is canonical. Web queries resolve to linked Telegram user when linked.
- **What values should gearbox store?** `"Automatic"` / `"Manual"` — matching the Yad2 scraper output (confirmed in `parser_test.go:54`). NOT abbreviated `"auto"/"manual"` which would break `EqualFold` filtering at `filter.go:63`.
- **What happens on unlink?** Data stays with the Telegram user. Unlink is a destructive operation requiring user confirmation. The web user starts fresh with no searches/bookmarks.
- **What if the linked Telegram user is deleted?** `resolveCanonicalChatID` falls back to the web user's own chatID. `GetLinkedTelegramUser` must check `active` status.
- **Does onDeleteSearch have a confirmation dialog?** No — verified at `callbacks.go:58-78`, it deletes immediately. Unit 4's confirmation pattern is new, not copied from an existing pattern.
- **Does the rate limit bypass (#1126) actually exist?** The existing nil check at `callbacks.go:36-38` returns early if `Message.Message` is nil, and `callbacks.go:50` rejects `chatID != fromID`. Investigate during implementation whether a gap remains; if both guards already cover the vector, close #1126 as not-a-bug.
- **Where is the config file?** `internal/config/config.go` (NOT `internal/app/config.go`).

### Deferred to Implementation

- **Exact typing indicator placement:** Which handlers are slow enough to warrant `SendChatAction`. Measure during implementation.
- **Quick-start config schema (#1138):** Exact YAML key names — follow existing `config.Enricher` pattern.

## Implementation Units

### Phase 1: Critical Bug Fixes (5 issues)

- [ ] **Unit 1: Fix race condition in wizard source selection (#1124)**

**Goal:** Eliminate the race window where the lock is released before sending the UI response in `onSourceDone` and `onLegacySourceSelected`.

**Requirements:** R1

**Dependencies:** None

**Files:**
- Modify: `internal/bot/wizard.go`
- Test: `internal/bot/wizard_test.go`

**Approach:**
- Restructure `onLegacySourceSelected` to NOT release the lock before calling `onSourceDone`. Instead, inline the source-done logic or pass the lock through.
- Ensure the full sequence (load state → modify → save → send keyboard) happens under a single `lockChat` hold.

**Patterns to follow:**
- `onManufacturerSelected` in `wizard.go` — correct pattern: lock held through entire handler

**Test scenarios:**
- Happy path: select source → manufacturer keyboard appears with correct data
- Integration: rapid concurrent callbacks from same chat don't corrupt wizard state (simulate two callbacks in sequence without unlock gap)

**Verification:**
- No race window between state save and keyboard send
- Existing wizard tests still pass

---

- [ ] **Unit 2: Surface saveWizardState and ensureUser failures to users (#1125)**

**Goal:** When `UpdateUserState` or `UpsertUser` fails, send the user an error message instead of silently continuing.

**Requirements:** R2

**Dependencies:** None

**Files:**
- Modify: `internal/bot/bot.go` (add `saveWizardStateOrAbort` helper, fix `ensureUser`)
- Modify: `internal/bot/wizard.go` (~23 call sites → one-line guard)
- Modify: `internal/bot/callbacks.go` (call sites)
- Modify: `internal/bot/commands.go` (call sites)
- Modify: `internal/locale/commands.go`
- Test: `internal/bot/error_paths_test.go`

**Approach:**
- Add `saveWizardStateOrAbort(ctx, chatID, lang, state, wd) bool` helper in `bot.go` that wraps `saveWizardState`, sends `locale.T(lang, "wizard_save_failed")` on failure, and returns false. Does NOT reset to idle (the lock is held, and the next user action will retry the DB write).
- Replace all ~35 `saveWizardState` calls with `if !b.saveWizardStateOrAbort(...) { return }`.
- In `ensureUser`, send generic error and return early if `UpsertUser` fails.
- Add locale keys: `"wizard_save_failed"` in both Hebrew and English.

**Patterns to follow:**
- Existing error handling in `handleStop` (sends specific error message on failure)

**Test scenarios:**
- Error path: `UpdateUserState` returns error → user receives "wizard_save_failed" message, state NOT reset to idle
- Error path: `UpsertUser` returns error → handler returns early, no further processing
- Error path: `saveWizardStateOrAbort` returns false → caller returns immediately
- Happy path: successful state save → no error message sent

**Verification:**
- `grep -c "saveWizardState(" internal/bot/wizard.go` returns 0 (all replaced by `saveWizardStateOrAbort`)
- No silent state persistence failures remain

---

- [ ] **Unit 3: Fix rate limit bypass for malformed callbacks (#1126)**

**Goal:** Prevent rate limit bypass when callback `chatID` cannot be extracted (defaults to 0).

**Requirements:** R3

**Dependencies:** None

**Files:**
- Modify: `internal/bot/bot.go`
- Test: `internal/bot/bot_test.go` or `internal/bot/coverage_test.go`

**Approach:**
- First verify if the bug actually exists: the nil check at `callbacks.go:36-38` returns early if `Message.Message` is nil, and `callbacks.go:50` rejects `chatID != fromID`. If both guards already cover the vector, close #1126 as not-a-bug.
- If a gap exists (e.g., `Message.Message` is non-nil but `Chat.ID` is 0), add an early return in the `rateLimited` middleware when `chatID` remains 0 after extraction.

**Patterns to follow:**
- Existing nil-check pattern at `callbacks.go:36-38`

**Test scenarios:**
- Edge case: callback with nil Message.Message → handler not invoked (verify existing guard suffices)
- Edge case: callback with Message.Message.Chat.ID == 0 → handler not invoked
- Happy path: callback with valid chatID → rate limiting applied normally

**Verification:**
- Either: confirm existing guards cover the vector and close #1126, OR add the fix and verify `isRateLimited(0)` is never called

---

- [ ] **Unit 4: Add confirmation dialog to "Clear Hidden" (#1127)**

**Goal:** Prevent accidental one-click clear of all hidden listings.

**Requirements:** R4

**Dependencies:** None

**Files:**
- Modify: `internal/bot/history.go`
- Modify: `internal/bot/keyboards.go`
- Modify: `internal/bot/callback_registry.go`
- Modify: `internal/locale/commands.go`
- Test: `internal/bot/callbacks_test.go`

**Approach:**
- Rename current `hidden_clear` callback to `hidden_clear_confirm`.
- Add new `hidden_clear` callback that shows a confirmation keyboard: "Clear N hidden listings?" with Yes/Cancel buttons.
- `hidden_clear_confirm` executes the actual clear.

**Patterns to follow:**
- This is a new confirmation pattern — no existing example in the codebase. Use two-callback approach: prompt callback shows keyboard, confirm callback executes.

**Test scenarios:**
- Happy path: click clear → confirmation prompt with count → confirm → listings cleared
- Edge case: click clear → confirmation prompt → cancel → listings preserved
- Edge case: click clear when zero hidden → appropriate message ("nothing to clear")

**Verification:**
- Cannot clear hidden listings with a single button press

---

- [ ] **Unit 5: Normalize gearbox to language-neutral codes (#1129)**

**Goal:** Store gearbox as `"Automatic"` / `"Manual"` (matching Yad2 scraper output) instead of Hebrew text.

**Requirements:** R5

**Dependencies:** None

**Files:**
- Modify: `internal/bot/keyboards.go` (callback data + display mapping function)
- Modify: `internal/bot/wizard.go`
- Modify: `internal/locale/wizard.go` (if display labels need mapping)
- Test: `internal/bot/wizard_test.go`

**Approach:**
- Change callback data from `cbPrefixGearBox + "אוטומט"` to `cbPrefixGearBox + "Automatic"` and `cbPrefixGearBox + "Manual"`. These values match the Yad2 scraper output (`parser_test.go:54`), so `EqualFold` filtering at `filter.go:63` and `LOWER` comparison at `listings.go:33` will work correctly.
- In `onGearBoxSelected`, map `"Automatic"` → store `"Automatic"`, `"Manual"` → store `"Manual"`, `"any"` → store `""`.
- Add a `gearboxDisplayName(lang, code)` helper in `keyboards.go` that maps codes and old Hebrew values to localized display labels. Use in `confirmKeyboard` to show the user a localized label instead of the raw code.
- The filter layer (`filter.go:63`) uses `EqualFold` which handles `"Automatic" == "automatic"` naturally. Old Hebrew values (`"אוטומט"`) in existing searches will NOT match new listings — this is acceptable because those searches will be updated when the user next edits them.
- Button labels remain localized via `locale.T()`.

**Patterns to follow:**
- Engine type callback uses English codes already (`"diesel"`, `"electric"`, etc.)

**Test scenarios:**
- Happy path: select auto → `WizardData.GearBox` is `"Automatic"`
- Happy path: select manual → `WizardData.GearBox` is `"Manual"`
- Happy path: confirmation summary shows localized label (e.g., "אוטומט" in Hebrew), not code
- Edge case: `gearboxDisplayName` handles old Hebrew values gracefully for display

**Verification:**
- No Hebrew text in callback data for new searches
- `"Automatic"` matches Yad2 scraper output via `EqualFold`

---

### Phase 2: Error Handling & UX Polish (4 issues)

- [ ] **Unit 6: Add feedback for stale/expired callbacks (#1128)**

**Goal:** When a user clicks a button from an expired wizard session, show a helpful message instead of silent ignore.

**Requirements:** R2

**Dependencies:** None

**Files:**
- Modify: `internal/bot/state.go`
- Modify: `internal/bot/wizard.go` (~10 call sites that check `expectState`)
- Modify: `internal/locale/commands.go`
- Test: `internal/bot/callbacks_test.go`

**Approach:**
- Change `expectState()` to return `(bool, error)` — `(true, nil)` on match, `(false, nil)` on mismatch, `(false, err)` on DB error.
- Add an `expectStateOrNotify(ctx, chatID, expected) bool` helper that wraps `expectState`, sends `locale.T(lang, "callback_expired")` on mismatch, sends `locale.T(lang, "error_generic")` and logs on DB error, returns false for both.
- Replace all ~10 `expectState` calls in wizard.go with `expectStateOrNotify`.

**Patterns to follow:**
- `onUnknownCallback` in `callback_registry.go` — already sends "callback_expired" for unknown callback data

**Test scenarios:**
- Edge case: click manufacturer button while in idle state → "callback expired" message
- Error path: DB error in expectState → generic error message sent, error logged
- Happy path: correct state → no error message, handler proceeds

**Verification:**
- No silent returns from `expectState` — every path has user feedback

---

- [ ] **Unit 7: Replace generic error messages with specific ones (#1133)**

**Goal:** Users see descriptive error messages instead of "Something went wrong."

**Requirements:** R2

**Dependencies:** None

**Files:**
- Modify: `internal/bot/wizard.go`
- Modify: `internal/locale/commands.go`
- Modify: `internal/locale/wizard.go`

**Approach:**
- Keywords too long (`wizard.go:603`): use `locale.Tf(lang, "error_keywords_too_long", maxKeywordsLen)` instead of `error_generic`
- Add 3-4 specific locale keys replacing uses of `error_generic` in wizard handlers

**Test scenarios:**
- Edge case: keywords exceeding 500 chars → specific "too long" message with character limit
- Happy path: keywords within limit → saved successfully

**Test expectation: none** — locale key changes are string-only; existing wizard tests cover the flow.

**Verification:**
- `grep -r "error_generic" internal/bot/wizard.go` returns zero matches

---

- [ ] **Unit 8: Fix inconsistent pagination error handling (#1139)**

**Goal:** `onSavedPage` and `onHiddenPage` handle parse errors the same way as `onHistoryPage`.

**Requirements:** R2

**Dependencies:** None

**Files:**
- Modify: `internal/bot/history.go`
- Test: `internal/bot/history_test.go` or `internal/bot/callbacks_test.go`

**Approach:**
- In `onSavedPage` and `onHiddenPage`, add logging and error message on `strconv.Atoi` failure, matching `onHistoryPage` pattern.

**Patterns to follow:**
- `onHistoryPage` at `history.go:121-129`

**Test scenarios:**
- Error path: corrupted saved_pg callback data → error logged, user sees error message
- Happy path: valid page number → page renders correctly

**Verification:**
- All three pagination handlers have identical error handling structure

---

- [ ] **Unit 9: Add wizard timeout mention and warning (#1134)**

**Goal:** Users know their wizard session will expire and aren't surprised by auto-cancel.

**Requirements:** R9

**Dependencies:** None

**Files:**
- Modify: `internal/locale/wizard.go`
- Modify: `internal/bot/wizard.go` (optional: reduce timeout)

**Approach:**
- Option A (simpler, recommended): Add timeout mention to the first wizard message: "Note: this setup expires after 30 minutes of inactivity."
- Update the `wizard_source_prompt` or `wizard_welcome` locale keys to include the timeout note.

**Test expectation: none** — locale string change only.

**Verification:**
- First wizard message mentions the timeout duration

---

### Phase 3: Notification UX (4 issues)

- [ ] **Unit 10: Add typing indicators (#1130)**

**Goal:** Bot shows "typing..." while processing commands that hit the DB.

**Requirements:** R6

**Dependencies:** None

**Files:**
- Modify: `internal/bot/messenger.go` (add `SendChatAction` to interface + implement on `telegramMessenger`)
- Modify: `internal/bot/bot.go` (add `sendTyping` helper)
- Modify: `internal/bot/commands.go` (add to slow handlers)
- Modify: `internal/bot/wizard.go` (add to manufacturer/model loading)
- Test: `internal/bot/testhelpers_test.go` (add `SendChatAction` to `mockMessenger`)

**Approach:**
- Add `SendChatAction(ctx, chatID, action)` to `messenger` interface.
- Add `b.sendTyping(ctx, chatID)` helper that calls `SendChatAction("typing")` and ignores errors.
- Call at the top of: `/list`, `/history`, `/saved`, `/hidden`, `/stats`, `/settings`, wizard manufacturer/model load.
- Mock implementation records calls but does nothing.

**Patterns to follow:**
- `b.sendMarkdown()` helper pattern — thin wrapper around messenger

**Test scenarios:**
- Happy path: `/list` command → `SendChatAction("typing")` called before response
- Integration: typing action failure → silently ignored, command still works

**Verification:**
- Slow commands send typing indicator before processing

---

- [ ] **Unit 11: Add save/hide buttons to notification messages (#1131)**

**Goal:** Users can save or hide listings directly from notification messages.

**Requirements:** R7

**Dependencies:** None

**Files:**
- Modify: `internal/notifier/telegram/telegram.go` (add `NotifyRawWithKeyboard` method)
- Modify: `internal/notifier/multi.go` (type-assert to `KeyboardNotifier` when keyboard present)
- Create: `internal/notifier/keyboard.go` (define `KeyboardNotifier` interface + keyboard builder, avoiding circular import with bot package)
- Test: `internal/notifier/telegram/telegram_test.go`

**Approach:**
- Define a `KeyboardNotifier` interface in `internal/notifier/keyboard.go` with `NotifyRawWithKeyboard(ctx, recipient, message, keyboard)`. The shared `Notifier` interface stays unchanged — no variadic options pollution.
- Implement `NotifyRawWithKeyboard` on `telegram.Notifier` only. WebPush has no keyboard concept.
- In `MultiNotifier`, when keyboard data is present, type-assert each resolved notifier to `KeyboardNotifier`. If it implements it, call `NotifyRawWithKeyboard`; otherwise fall back to `NotifyRaw`.
- Move the keyboard builder logic to `internal/notifier/keyboard.go` to avoid circular imports between `bot` and `notifier` packages. The listing token is the only input needed.
- For batch messages (multiple listings), skip keyboard (too many listings to act on).
- Listing tokens are semi-public (they appear in Yad2 URLs), so any user saving/hiding any token is an acceptable operation — not a security concern.

**Patterns to follow:**
- Type-assertion pattern used throughout Go stdlib (e.g., `io.WriterTo`)

**Test scenarios:**
- Happy path: single listing notification → Telegram message has save/hide inline keyboard
- Happy path: batch notification → no inline keyboard
- Integration: user clicks save button on notification → listing saved via existing `save:` callback
- Edge case: WebPush notifier → `NotifyRaw` called (no keyboard), no error

**Test scenarios:**
- Happy path: single listing notification → message has save/hide inline keyboard
- Happy path: batch notification → no inline keyboard (too many listings)
- Integration: user clicks save button on notification → listing saved, button callback works
- Edge case: notification to web-only user → keyboard ignored (webpush doesn't support keyboards)

**Verification:**
- Single-listing Telegram notifications include actionable buttons

---

- [ ] **Unit 12: Cap notification batch size (#1132)**

**Goal:** Prevent flooding users with 20+ consecutive messages when a broad search matches many listings.

**Requirements:** R8

**Dependencies:** None

**Files:**
- Modify: `internal/scheduler/delivery.go` (cap listing slice before passing to notifier)
- Modify: `internal/notifier/formatter.go` (add footer when truncated)
- Modify: `internal/locale/commands.go` (localized footer message)
- Test: `internal/notifier/formatter_test.go`
- Test: `internal/scheduler/delivery_test.go`

**Approach:**
- Add `maxBatchSize = 10` constant in the scheduler delivery layer.
- Cap the listing slice BEFORE passing to `Notifier.Notify` — this prevents both text batch flooding AND individual photo message flooding.
- Pass a `truncated int` count to `FormatBatch` so it can append a localized footer: "...and N more new listings. Use /history to see all."
- The formatter handles the display; the scheduler handles the enforcement.

**Patterns to follow:**
- Existing batch separator pattern in `formatter.go`

**Test scenarios:**
- Happy path: 5 listings → all 5 formatted, no footer
- Edge case: 15 listings → 10 formatted + footer showing "5 more"
- Edge case: exactly 10 listings → all 10 formatted, no footer

**Verification:**
- No notification message contains more than 10 listing summaries

---

- [ ] **Unit 13: Fix silent web-only notification failures (#1137)**

**Goal:** Web-only users without webpush subscriptions still know about new matches.

**Requirements:** R10

**Dependencies:** None

**Files:**
- Modify: `internal/notifier/multi.go`
- Modify: `internal/api/notifications.go` (ensure notification center stores undelivered matches)
- Test: `internal/notifier/multi_test.go`

**Approach:**
- In `resolveAll()`, when a web user has no working notification channel, log a warning with the user's chatID.
- Ensure the notification center (API-side) always records the notification regardless of push delivery success — users can see matches when they open the dashboard.
- If no channels resolve successfully, return `ErrNoChannelNotifier` (already handled by the dead-letter hook to clean up dedup claims).

**Patterns to follow:**
- `ErrNoChannelNotifier` handling in `consumer.go:218-228`

**Test scenarios:**
- Error path: web user with no webpush subscription → `ErrNoChannelNotifier` returned, notification stored in DB
- Happy path: web user with active subscription → webpush delivered
- Edge case: linked user → both telegram and webpush attempted

**Verification:**
- No silent notification failures for web-only users

---

### Phase 4: Web-Bot Sync & Polish (4 issues)

- [ ] **Unit 14: Make quick-start configurable (#1138)**

**Goal:** Quick-start manufacturer/model comes from config instead of hardcoded constants.

**Requirements:** R13

**Dependencies:** None

**Files:**
- Modify: `internal/bot/callbacks.go`
- Modify: `internal/config/config.go` (add `Bot.QuickStartManufacturer`, `Bot.QuickStartModel`)
- Modify: `internal/bot/bot.go` (accept config)
- Test: `internal/bot/callbacks_test.go`

**Approach:**
- Add config fields with defaults matching current values (19/8640 = Toyota Corolla).
- Pass through Bot struct on initialization.

**Patterns to follow:**
- `cfg.Enricher.BaseDelay` pattern — config fields with sensible defaults

**Test scenarios:**
- Happy path: quick-start with default config → Toyota Corolla search created
- Happy path: quick-start with custom config → custom search created

**Verification:**
- No hardcoded manufacturer/model IDs in `callbacks.go`

---

- [ ] **Unit 15: Sync searches between linked accounts (#1135)**

**Goal:** When Telegram and web accounts are linked, both see the same searches.

**Requirements:** R11

**Dependencies:** None

**Files:**
- Modify: `internal/storage/postgres/users.go` (enhance `LinkTelegramToWeb` to migrate searches; `GetLinkedTelegramUser` checks `active` status)
- Modify: `internal/api/searches.go` (resolve chatID through link)
- Modify: `internal/api/api.go` (add `resolveCanonicalChatID` helper)
- Test: `internal/storage/postgres/postgres_test.go`
- Test: `internal/api/api_test.go`

**Approach:**
- On link creation (`LinkTelegramToWeb`), migrate all web user's searches to the Telegram user's `chat_id` within the same transaction. Skip duplicates: if the Telegram user already has a search for the same manufacturer+model, skip the web user's duplicate.
- In the API layer, add a `resolveCanonicalChatID(ctx)` helper: if the authenticated web user has a `linked_web_id` relationship with an **active** Telegram user, return the Telegram `chat_id` instead. If the linked Telegram user is deleted or inactive, fall back to the web user's own chatID.
- `GetLinkedTelegramUser` must add `AND active = true` to its query.
- Unlink behavior: data stays with the Telegram user. Unlink requires user confirmation ("Your searches and saved listings will remain with your Telegram account"). The web user starts fresh.

**Patterns to follow:**
- Transaction pattern in `LinkTelegramToWeb`
- `chatIDFromContext()` in `api.go`

**Test scenarios:**
- Happy path: link accounts → web user's searches become visible in bot
- Happy path: create search on web after linking → search visible in bot
- Edge case: link accounts when both have searches → non-duplicate searches merged, duplicates skipped
- Edge case: unlink → searches stay with Telegram user, web user sees empty list
- Edge case: linked Telegram user deleted → web API falls back to web user's own chatID
- Edge case: linked Telegram user set to `active=false` → web API falls back
- Edge case: unlink then relink to different Telegram user → data stays with old Telegram user
- Error path: link transaction fails → no partial migration

**Verification:**
- After linking, `/list` in bot shows web-created searches
- After linking, web dashboard shows bot-created searches
- After Telegram user deletion, web user retains access to their own data

---

- [ ] **Unit 16: Sync bookmarks and settings (#1136)**

**Goal:** Saved/hidden listings, language, digest mode, and tier are shared between linked accounts.

**Requirements:** R12

**Dependencies:** Unit 15 (same `resolveCanonicalChatID` mechanism)

**Files:**
- Modify: `internal/api/bookmarks.go` (use resolved chatID)
- Modify: `internal/api/api.go` (extend resolver to cover bookmark endpoints)
- Modify: `internal/storage/postgres/users.go` (migrate bookmarks on link, if any exist)
- Test: `internal/api/api_test.go`

**Approach:**
- Same `resolveCanonicalChatID` helper from Unit 15 applied to bookmark and settings endpoints.
- On link creation, migrate existing web bookmarks to the Telegram user's chatID.
- Language and digest settings: API reads/writes use the resolved chatID, so changes on web automatically affect bot behavior.

**Patterns to follow:**
- Unit 15's resolution pattern

**Test scenarios:**
- Happy path: save listing on web → appears in bot's /saved
- Happy path: change language on web → bot uses new language
- Happy path: change digest mode on bot → web reflects new mode
- Edge case: both accounts have saved listings → merged on link

**Verification:**
- After linking, all user-scoped data uses a single canonical chatID

---

- [ ] **Unit 17: Add change preview in /edit confirmation (#1140)**

**Goal:** When editing a search, the confirmation screen highlights what changed.

**Requirements:** R14

**Dependencies:** None

**Files:**
- Modify: `internal/bot/commands.go` (store original search params in WizardData on /edit start)
- Modify: `internal/bot/keyboards.go` (confirmation keyboard builder adds diff section)
- Modify: `internal/botcore/state.go` (add `OriginalSearch` field to WizardData)
- Modify: `internal/locale/wizard.go` (add diff format strings)
- Test: `internal/bot/commands_test.go`

**Approach:**
- When `/edit` starts, snapshot the current search params into a new `WizardData.OriginalSearch` field.
- At confirmation time, compare each field (year min/max, price, gearbox, etc.) and format only the changed fields as "Field: old → new".
- Append the diff below the standard confirmation summary.

**Patterns to follow:**
- `confirmKeyboard` builder in `keyboards.go`

**Test scenarios:**
- Happy path: edit year min from 2018→2020 → confirmation shows "Year min: 2018 → 2020"
- Edge case: edit with no changes → confirmation shows "No changes detected"
- Happy path: multiple fields changed → all listed in diff

**Verification:**
- `/edit` confirmation includes a diff section showing changed fields

## System-Wide Impact

- **Interaction graph:** Unit 11 adds a `KeyboardNotifier` interface (type-assertion, not interface change) — only `telegram.Notifier` implements it. Units 15-16 touch the API auth resolution path via `resolveCanonicalChatID` — all authenticated endpoints affected.
- **Error propagation:** Unit 2 adds `saveWizardStateOrAbort` helper — ~35 call sites in wizard.go, callbacks.go, commands.go become one-line guards.
- **State lifecycle risks:** Unit 5 (gearbox) stores `"Automatic"/"Manual"` matching scraper output — old Hebrew values in existing searches will not match new listings but won't error. Unit 15 (search migration) must be transactional with duplicate skipping.
- **API surface parity:** Unit 15's `resolveCanonicalChatID` affects all API endpoints that use `chatIDFromContext` — must fall back to web chatID when linked Telegram user is deleted/inactive.
- **Unchanged invariants:** Redis Streams protocol, enrichment pipeline, scraper feed cycle, Telegram bot API version, and the shared `Notifier` interface are not changed.

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Unit 2 (saveWizardState) touches ~35 call sites | `saveWizardStateOrAbort` helper reduces each to a one-line guard |
| Unit 5 (gearbox) old Hebrew searches stop matching | Acceptable — users update searches via /edit; no silent data loss |
| Unit 11 (notification keyboards) circular import risk | Keyboard builder in `internal/notifier/keyboard.go` avoids bot→notifier cycle |
| Units 15-16 (sync) linked Telegram user deleted | `resolveCanonicalChatID` falls back to web chatID; tested explicitly |
| Units 15-16 (sync) unlink loses web user's data | Unlink requires confirmation; documented as destructive operation |
| Batch of 17 issues is large | Phased delivery — each phase ships independently with its own PR |

## Phased Delivery

### Phase 1: Critical Bugs (Units 1-5)
Ship as one PR. All are independent, low-risk, well-bounded fixes. Closes #1124, #1125, #1126, #1127, #1129.

### Phase 2: Error Handling & UX Polish (Units 6-9)
Ship as one PR. Improves user-facing messaging. Closes #1128, #1133, #1139, #1134.

### Phase 3: Notification UX (Units 10-13)
Ship as 1-2 PRs. Unit 11 (action buttons) is the most complex. Closes #1130, #1131, #1132, #1137.

### Phase 4: Web-Bot Sync & Polish (Units 14-17)
Ship as 2 PRs: Units 14+17 (config/edit preview) and Units 15+16 (sync). Closes #1135, #1136, #1138, #1140.

## Sources & References

- Related issues: #1124, #1125, #1126, #1127, #1128, #1129, #1130, #1131, #1132, #1133, #1134, #1135, #1136, #1137, #1138, #1139, #1140
- Key files: `internal/bot/wizard.go`, `internal/bot/bot.go`, `internal/bot/callbacks.go`, `internal/bot/keyboards.go`, `internal/bot/history.go`, `internal/bot/state.go`, `internal/bot/callback_registry.go`, `internal/notifier/multi.go`, `internal/notifier/telegram/telegram.go`, `internal/notifier/formatter.go`, `internal/storage/postgres/users.go`, `internal/api/searches.go`
