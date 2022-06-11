CREATE TYPE valid_status AS ENUM ('new', 'success', 'failure', 'error');
CREATE TYPE valid_currency AS ENUM ('usd', 'eur', 'rub');

CREATE TABLE payments (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL,
  email VARCHAR (20) NOT NULL,
  amount NUMERIC(10, 2) NOT NULL CHECK (amount > 0),
  currency valid_currency NOT NULL,
  payment_status valid_status NOT NULL DEFAULT 'new',
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX ON payments (email);
CREATE INDEX ON payments (user_id);