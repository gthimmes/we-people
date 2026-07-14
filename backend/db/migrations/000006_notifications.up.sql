-- In-app notifications (email delivery layered on later).
CREATE TABLE notifications (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id            uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    recipient_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type              text NOT NULL,          -- e.g. approval.assigned, approval.decided
    title             text NOT NULL,
    body              text NOT NULL DEFAULT '',
    link              text NOT NULL DEFAULT '', -- frontend path to open
    read_at           timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_recipient
    ON notifications(recipient_user_id, created_at DESC);
CREATE INDEX idx_notifications_unread
    ON notifications(recipient_user_id) WHERE read_at IS NULL;
