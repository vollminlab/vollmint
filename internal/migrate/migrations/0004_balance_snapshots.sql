-- +goose Up
CREATE TABLE account_balance_snapshots (
    account_id    text NOT NULL REFERENCES accounts(id),
    snapshot_date date NOT NULL,
    balance       numeric(14,2) NOT NULL,
    PRIMARY KEY (account_id, snapshot_date)
);

ALTER TABLE accounts ADD COLUMN is_manual boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE accounts DROP COLUMN is_manual;
DROP TABLE account_balance_snapshots;
