CREATE TABLE public.users (
    id VARCHAR(255) PRIMARY KEY,
    first_name VARCHAR(255),
    second_name VARCHAR(255),
    birthdate DATE,
    sex VARCHAR(255),
    biography TEXT,
    city VARCHAR(255),
    username VARCHAR(255) NOT NULL,
    password BYTEA NOT NULL,
    UNIQUE (username)
);
