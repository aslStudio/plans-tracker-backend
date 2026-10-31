ALTER TABLE apple_identities ADD COLUMN email_forwarding_enabled boolean NOT NULL DEFAULT true;

CREATE TABLE apple_notification_receipts (
    jti text PRIMARY KEY,
    received_at timestamptz NOT NULL DEFAULT now()
);
