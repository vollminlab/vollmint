-- +goose Up
CREATE TABLE accounts (
    id           text PRIMARY KEY,
    name         text NOT NULL,
    org          text NOT NULL DEFAULT '',
    currency     text NOT NULL DEFAULT 'USD',
    owner        text NOT NULL CHECK (owner IN ('scott','nikki','joint')),
    balance      numeric(14,2),
    balance_date date,
    active       boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE categories (
    id        serial PRIMARY KEY,
    name      text NOT NULL UNIQUE,
    parent_id int REFERENCES categories(id),
    kind      text NOT NULL DEFAULT 'spend' CHECK (kind IN ('spend','income','transfer','savings')),
    is_vice   boolean NOT NULL DEFAULT false
);

CREATE TABLE transactions (
    id               bigserial PRIMARY KEY,
    source           text NOT NULL CHECK (source IN ('simplefin','venmo_csv')),
    external_id      text NOT NULL,
    account_id       text NOT NULL REFERENCES accounts(id),
    posted           date NOT NULL,
    amount           numeric(12,2) NOT NULL,
    description      text NOT NULL DEFAULT '',
    payee            text NOT NULL DEFAULT '',
    pending          boolean NOT NULL DEFAULT false,
    category_id      int REFERENCES categories(id),
    owner_override   text CHECK (owner_override IN ('scott','nikki','joint')),
    transfer_peer_id bigint REFERENCES transactions(id),
    raw              jsonb NOT NULL DEFAULT '{}',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source, external_id)
);
CREATE INDEX idx_txn_posted   ON transactions (posted);
CREATE INDEX idx_txn_category ON transactions (category_id);
CREATE INDEX idx_txn_account  ON transactions (account_id);

CREATE TABLE category_rules (
    id          serial PRIMARY KEY,
    priority    int NOT NULL,
    match_type  text NOT NULL DEFAULT 'substring' CHECK (match_type IN ('substring','regex')),
    pattern     text NOT NULL,
    category_id int NOT NULL REFERENCES categories(id)
);

CREATE TABLE budgets (
    category_id int NOT NULL REFERENCES categories(id),
    month       date NOT NULL,
    amount      numeric(12,2) NOT NULL,
    PRIMARY KEY (category_id, month)
);

CREATE TABLE sync_runs (
    id            bigserial PRIMARY KEY,
    kind          text NOT NULL CHECK (kind IN ('simplefin','venmo_csv')),
    started       timestamptz NOT NULL DEFAULT now(),
    finished      timestamptz,
    status        text NOT NULL DEFAULT 'running' CHECK (status IN ('running','ok','partial','failed')),
    window_start  date,
    window_end    date,
    rows_upserted int NOT NULL DEFAULT 0,
    detail        text NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE sync_runs; DROP TABLE budgets; DROP TABLE category_rules;
DROP TABLE transactions; DROP TABLE categories; DROP TABLE accounts;
