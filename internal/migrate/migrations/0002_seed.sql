-- +goose Up
INSERT INTO categories (name, kind, is_vice) VALUES
    ('Housing',            'spend',    false),
    ('Groceries',          'spend',    false),
    ('Dining',             'spend',    true),
    ('Transport',          'spend',    false),
    ('Utilities',          'spend',    false),
    ('Subscriptions',      'spend',    false),
    ('Entertainment',      'spend',    false),
    ('Shopping',           'spend',    false),
    ('Health',             'spend',    false),
    ('Travel',             'spend',    false),
    ('Vices',              'spend',    true),
    ('Paycheck',           'income',   false),
    ('Savings',            'savings',  false),
    ('Transfer',           'transfer', false),
    ('Needs Venmo detail', 'spend',    false);

-- Venmo has no SimpleFIN feed; CSV imports attach to this synthetic account.
INSERT INTO accounts (id, name, org, owner) VALUES ('venmo', 'Venmo', 'Venmo', 'scott');

-- Ally-side Venmo ACH debits: until a CSV pairs them, they are spend in the
-- "needs detail" bucket (spec: totals never understate).
INSERT INTO category_rules (priority, match_type, pattern, category_id)
    SELECT 1000, 'substring', 'VENMO', id FROM categories WHERE name = 'Needs Venmo detail';

-- +goose Down
DELETE FROM category_rules WHERE priority = 1000 AND pattern = 'VENMO';
DELETE FROM accounts WHERE id = 'venmo';
DELETE FROM categories WHERE name IN (
    'Housing', 'Groceries', 'Dining', 'Transport', 'Utilities',
    'Subscriptions', 'Entertainment', 'Shopping', 'Health', 'Travel',
    'Vices', 'Paycheck', 'Savings', 'Transfer', 'Needs Venmo detail');
