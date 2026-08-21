# Notification baseline

The scaffold exposes `internal/app/notification.Sender` as a provider-neutral
boundary. `internal/adapters/mailer.SMTPClient` is the built-in SMTP adapter
slot for password notices, approval notifications, and operational alerts.

The adapter has no plaintext fallback: it requires TLS 1.2 or newer, validates
the configured server certificate, and supports STARTTLS or explicitly selected
implicit TLS. It rejects header injection, duplicate/oversized recipient lists,
oversized messages, and messages without a text or HTML body. HTML messages are
multipart/alternative and all delivery should be initiated by the reliable
message worker rather than inside a business transaction.

The local SMTP contract proves the adapter protocol boundary only. A production
deployment must still provide an approved SMTP/enterprise mail target, sender
authorization, TLS/mTLS and CA evidence, anti-spam policy, bounce handling,
template review, delivery audit, retention, and rate/quota controls. No mail
vendor is `Target-tested` by the local test.

`make mail-runtime-contract` starts the disposable domestic-source Mailpit
overlay at `deploy/compose/mail-runtime-contract.yaml`, submits one message over
SMTP, verifies it through the local HTTP API, and writes an `Experimental`
evidence record. The overlay is explicitly for development and contract tests;
it must not be used as a production mail relay.

For business-triggered mail, use `notificationdelivery.EmailQueue.EnqueueEmailTx`
inside the business transaction and register `EmailHandler.MessageHandler()` in
the existing `messageworker`. The payload binds a stable notification ID and
rejects mismatched envelopes. The worker provides retry, lease recovery, and
dead-letter behavior, so the delivery contract is at-least-once; a target mail
system or downstream consumer must tolerate duplicate delivery.
