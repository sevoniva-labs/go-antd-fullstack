-- +goose Up
ALTER TABLE organizations
  ADD COLUMN updated_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6);

-- +goose Down
ALTER TABLE organizations
  DROP COLUMN updated_at;
