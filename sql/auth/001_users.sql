CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- id unique de l'utilisateur
    username TEXT UNIQUE NOT NULL, -- nom d'utilisateur unique
    email TEXT UNIQUE NOT NULL, -- email unique
    email_verified BOOLEAN DEFAULT FALSE, -- email vérifié
    phone TEXT UNIQUE, -- numéro de téléphone unique
    phone_verified BOOLEAN DEFAULT FALSE, -- numéro de téléphone vérifié
    password_hash TEXT NOT NULL, -- mot de passe haché
    first_name TEXT NOT NULL, -- prénom
    last_name TEXT NOT NULL, -- nom de famille
    birthdate DATE, -- date de naissance
    sex SMALLINT, -- sexe
    bio TEXT, -- biographie
    profile_picture_id UUID, -- id de l'image de profil
    grade SMALLINT NOT NULL DEFAULT 1, -- grade de l'utilisateur
    location TEXT, -- localisation de l'utilisateur
    school TEXT, -- école
    work TEXT, -- emplois
    badges TEXT[], -- badges
    created_at TIMESTAMPTZ DEFAULT now(), -- date de création
    updated_at TIMESTAMPTZ DEFAULT now() -- date de mise à jour
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
