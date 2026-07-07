CREATE TABLE IF NOT EXISTS users (
    id            SERIAL PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    username      VARCHAR(50),
    email         VARCHAR(100) UNIQUE NOT NULL,
    phone         VARCHAR(15) UNIQUE NOT NULL,
    age           INT,
    profile_photo_path TEXT,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMP DEFAULT NOW(),
    updated_at    TIMESTAMP DEFAULT NOW()
);

-- Ensure name column exists (fix for existing databases)
ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR(100);
UPDATE users SET name = 'User' || id::text WHERE name IS NULL;
ALTER TABLE users ALTER COLUMN name SET NOT NULL;

ALTER TABLE users ADD COLUMN IF NOT EXISTS username VARCHAR(50);
ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_photo_path TEXT;

UPDATE users
SET username = 'user_' || id::text
WHERE username IS NULL OR username = '';

ALTER TABLE users ALTER COLUMN username SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone ON users(phone);

CREATE TABLE IF NOT EXISTS otp_codes (
    id         SERIAL PRIMARY KEY,
    email      VARCHAR(100) NOT NULL,
    code       CHAR(6) NOT NULL,
    purpose    VARCHAR(30) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used       BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_otp_email_purpose ON otp_codes(email, purpose);
