CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,              -- Modifié : BIGINT pur (correspond à int64)
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    email_verified BOOLEAN,             -- Modifié : Plus de valeur par défaut
    phone TEXT UNIQUE,
    phone_verified BOOLEAN,             -- Modifié : Plus de valeur par défaut
    password_hash TEXT NOT NULL,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    birthdate DATE,
    sex SMALLINT,
    bio TEXT,
    profile_picture_id BIGINT,
    grade SMALLINT NOT NULL,            -- Modifié : Plus de valeur par défaut (0)
    location TEXT,
    school TEXT,
    work TEXT,
    badges TEXT[],
    desactivated BOOLEAN,               -- Modifié : Plus de valeur par défaut
    banned BOOLEAN,                     -- Modifié : Plus de valeur par défaut
    ban_reason TEXT,
    ban_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,             -- Modifié : Plus de DEFAULT now()
    updated_at TIMESTAMPTZ              -- Modifié : Plus de DEFAULT now()
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_phone ON users(phone);