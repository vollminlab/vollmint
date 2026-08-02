-- +goose Up
CREATE TABLE transaction_splits (
    id             bigserial PRIMARY KEY,
    transaction_id bigint NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    category_id    int NOT NULL REFERENCES categories(id),
    amount         numeric(12,2) NOT NULL,
    note           text NOT NULL DEFAULT ''
);
CREATE INDEX idx_split_txn ON transaction_splits (transaction_id);
CREATE INDEX idx_split_category ON transaction_splits (category_id);

-- +goose Down
DROP TABLE transaction_splits;
