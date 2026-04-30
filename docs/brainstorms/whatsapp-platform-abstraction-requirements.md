# WhatsApp Platform Support — Requirements

**Date:** 2026-04-26
**Status:** Draft
**Issues:** #183, #77

## Problem

CarWatch only works via Telegram. WhatsApp penetration in Israel is ~95% vs ~30% for Telegram. Most potential users won't install Telegram for a car bot. Adding WhatsApp as a second channel immediately expands the addressable market.

## Goals

1. Users can onboard and manage car searches entirely through WhatsApp (no Telegram required)
2. Existing Telegram users are unaffected — zero regressions
3. Both channels run simultaneously from the same deployment
4. The architecture supports adding future channels (email, Discord) with minimal effort

## Non-Goals

- Dual-channel delivery (same user receiving on both Telegram AND WhatsApp) — deferred to later phase
- Migrating existing Telegram users to WhatsApp
- WhatsApp group support
- Rich media (images/photos) in WhatsApp messages — text-only for MVP

## Decisions

### User Identity: Synthetic int64 IDs with Range Separation

WhatsApp users get synthetic int64 IDs. The phone number is stored as metadata. This avoids changing 37+ storage method signatures.

**ID range strategy:** WhatsApp user IDs start at `1_000_000_000_000` (1 trillion). Telegram chat IDs for users are typically in the `100_000_000`–`9_999_999_999` range. This prevents collisions without needing negative numbers.

**Schema additions:**
- `users.channel` — `TEXT NOT NULL DEFAULT 'telegram'` (values: `telegram`, `whatsapp`)
- `users.channel_id` — `TEXT NOT NULL DEFAULT ''` (phone number for WhatsApp, empty for Telegram where chat_id IS the channel ID)

**Lookup-or-create flow for WhatsApp:** New method `UpsertWhatsAppUser(ctx, phoneNumber) (int64, error)` that atomically looks up the user by `channel='whatsapp' AND channel_id=phoneNumber`, or creates a new one with a generated ID in the WhatsApp range.

### Architecture: Extract Shared Core

```
internal/
  botcore/          ← NEW: shared business logic
  │  wizard.go         state machine, validation, transitions
  │  state.go          WizardData, state constants
  │  actions.go        callback action parsing (string prefix routing)
  │  presenter.go      Presenter interface (platform adapter contract)
  │
  bot/              ← EXISTING: Telegram adapter (minimal changes)
  │  messenger.go      Telegram messenger (unchanged)
  │  keyboards.go      Telegram inline keyboards (unchanged)
  │  handlers.go       routes tgmodels.Update → botcore actions
  │  presenter.go      implements botcore.Presenter for Telegram
  │
  whatsapp/         ← NEW: WhatsApp adapter
  │  handler.go        webhook handler, routes messages → botcore actions
  │  menus.go          text-based menu rendering
  │  presenter.go      implements botcore.Presenter for WhatsApp
  │  client.go         Meta Cloud API client (send messages, templates)
  │
  notifier/
  │  notifier.go     existing interface (unchanged)
  │  formatter.go    existing Telegram Markdown formatter (unchanged)
  │  multi.go        ← NEW: multiplexing notifier (routes by user channel)
  │  telegram/       existing (unchanged)
  │  whatsapp/       ← NEW: implements Notifier interface
```

### Core Abstraction: The Presenter Interface

The key abstraction enabling the shared wizard state machine. Each platform implements this interface:

```go
// botcore/presenter.go
type Presenter interface {
    SendText(ctx context.Context, chatID int64, text string) error
    SendChoices(ctx context.Context, chatID int64, prompt string, choices []Choice) error
    SendConfirmation(ctx context.Context, chatID int64, summary string) error
}

type Choice struct {
    Label string // display text ("Toyota", "2018", "Skip")
    Data  string // callback identifier ("mfr:27", "eng:2000", "skip_keywords")
}
```

- Telegram adapter: converts `[]Choice` → `*tgmodels.InlineKeyboardMarkup`
- WhatsApp adapter: converts `[]Choice` → numbered text menu ("1. Toyota\n2. Mazda\n...")
- Wizard state machine calls `Presenter` methods — never constructs platform-specific UI

### Notifier Dispatch: Multiplexing Notifier

The scheduler currently injects a single `notifier.Notifier`. For multi-channel support:

```go
// notifier/multi.go
type MultiNotifier struct {
    telegram  Notifier
    whatsapp  Notifier
    userStore storage.UserStore
}

func (m *MultiNotifier) Notify(ctx context.Context, recipient string, listings []model.Listing, lang locale.Lang) error {
    chatID, _ := strconv.ParseInt(recipient, 10, 64)
    user, _ := m.userStore.GetUser(ctx, chatID)
    switch user.Channel {
    case "whatsapp":
        return m.whatsapp.Notify(ctx, recipient, listings, lang)
    default:
        return m.telegram.Notify(ctx, recipient, listings, lang)
    }
}
```

This wraps both notifiers and routes by `users.channel`. The scheduler and delivery strategies remain unchanged — they just receive the `MultiNotifier` instead of the Telegram notifier.

### Message Formatting: Per-Platform Renderers

The existing `FormatBatch`/`FormatListing` produce Telegram Markdown (escaped `[`, `]` etc.). WhatsApp formatting is different.

**For proactive notifications (listing alerts):** WhatsApp uses template messages which require structured parameters, not pre-formatted text. The `Notifier.Notify` interface receives raw `[]model.Listing` — the WhatsApp notifier extracts fields directly to fill template parameters. The Telegram `FormatBatch` path is not used for WhatsApp.

**For interactive messages (wizard, commands):** WhatsApp uses `*bold*` and plain URLs. A lightweight `FormatWhatsApp(text)` helper strips Telegram-specific escaping and adjusts link formatting.

### WhatsApp Onboarding: WhatsApp-First

New users message the WhatsApp business number directly. The bot walks them through a text-based wizard (same state machine as Telegram, different UI rendering via Presenter). No Telegram account required.

### WhatsApp API: Meta Cloud API Direct

Use Meta's Cloud API directly (no BSP). The Go library [`piusalfred/whatsapp`](https://github.com/piusalfred/whatsapp) provides a mature, MIT-licensed client. Direct integration avoids BSP markup costs.

### Webhook Endpoint

Add `/webhook/whatsapp` to the existing HTTP mux (same port as `/healthz` and `/api/v1/`). No separate port needed for a single-instance SQLite app.

### Pricing Impact

- **Service messages (user-initiated) are free** within a 24-hour window — covers wizard flow and all interactive use
- **Template messages** (proactive alerts) cost ~$0.004/msg (utility category, Israel)
- At 50 users x 5 alerts/day = 250 msgs/day = ~$1/day = **~$30/month**
- No free tier for template messages since July 2025

## User Flows

### WhatsApp Onboarding
1. User sends any message to the WhatsApp business number
2. Bot replies with welcome text + "Reply 1 to set up a car search" (free service message within 24h window)
3. Text-based wizard: manufacturer → model → year → price → engine → keywords → confirm
4. Each step shows numbered options (e.g., "1. Toyota  2. Mazda  3. Honda  Type a name to search")
5. User replies with number or text
6. On confirm, search is created and polling begins

### WhatsApp Notifications
1. Scheduler finds new listings matching user's search
2. MultiNotifier routes to WhatsApp notifier based on `users.channel`
3. WhatsApp notifier sends via Cloud API **template message** (with listing data as template parameters)
4. Links rendered as plain URLs (WhatsApp auto-previews them)

### WhatsApp Commands
Text-based equivalents of Telegram commands:
- "menu" or "help" → show available actions
- "list" → show active searches
- "stop 1" → delete search #1
- "new" or "watch" → start wizard
- "edit 1" → edit search #1

## Constraints

- **24-hour window**: Bot can only send free-form messages within 24h of user's last message. Proactive alerts require pre-approved template messages.
- **Template approval**: Message templates must be submitted to Meta for review (1-3 business days). Required templates: new listing alert, price drop alert, daily digest.
- **No inline buttons**: WhatsApp interactive messages support "reply buttons" (max 3) and "list messages" (max 10 items with sections).
- **Rate limits**: Cloud API supports up to 500 messages/second per WABA.
- **Formatting**: WhatsApp supports `*bold*`, `_italic_`, `~strikethrough~`, `` `monospace` ``. No Markdown links — URLs must be plain text.
- **Meta compliance**: Bots must perform concrete business tasks. Open-ended AI chat is prohibited.

## Success Criteria

- [ ] A new user can onboard via WhatsApp and receive their first car listing alert
- [ ] All Telegram functionality continues working unchanged
- [ ] WhatsApp and Telegram users share the same backend (searches, dedup, scheduler)
- [ ] Template messages approved by Meta for listing alerts
- [ ] Template messages used for proactive notifications; free-form messages used for wizard interactions

## Phasing

### Phase 3A: Platform Abstraction (code refactoring, no WhatsApp yet)
- Define `botcore.Presenter` interface
- Extract wizard state machine, WizardData, state constants, and callback routing to `internal/botcore/`
- Implement `bot.TelegramPresenter` that wraps the existing messenger + keyboards
- Refactor `internal/bot/` handlers to use `botcore.Presenter` instead of calling messenger directly
- Add `users.channel` and `users.channel_id` columns (migration)
- Add `UpsertWhatsAppUser` storage method
- Create `notifier.MultiNotifier` (initially wraps only Telegram)
- **All Telegram tests must continue passing — zero behavior change**

### Phase 3B: WhatsApp MVP
- WhatsApp webhook handler (`/webhook/whatsapp` endpoint)
- Meta Cloud API client via `piusalfred/whatsapp`
- WhatsApp notifier (implements `Notifier` interface, uses template messages)
- `whatsapp.Presenter` (text-based menus, numbered options)
- WhatsApp message routing → botcore actions
- Submit message templates to Meta for approval
- Wire MultiNotifier with both Telegram and WhatsApp notifiers

### Phase 3C: Polish
- WhatsApp interactive messages (reply buttons for confirm/cancel)
- Daily digest via WhatsApp template
- Saved/hidden listings support
- Graceful handling of template delivery failures
- User analytics by channel

## Open Questions

- What WhatsApp business phone number to use? (needs Meta Business verification)

## References

- [WhatsApp Cloud API Pricing 2026](https://business.whatsapp.com/products/platform-pricing)
- [WhatsApp API Compliance 2026](https://gmcsco.com/your-simple-guide-to-whatsapp-api-compliance-2026/)
- [piusalfred/whatsapp Go library](https://github.com/piusalfred/whatsapp)
- [WhatsApp Bot Development Guide 2026](https://www.groovyweb.co/blog/whatsapp-business-bot-development-2026)
- [Meta Cloud API Developer Docs](https://developers.facebook.com/documentation/business-messaging/whatsapp/pricing)
- [WhatsApp Cloud API Integration Guide](https://medium.com/@aktyagihp/whatsapp-cloud-api-integration-in-2026-0493dd05d644)
