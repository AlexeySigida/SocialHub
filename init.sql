CREATE TABLE public.users (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(255),
    second_name VARCHAR(255),
    birthdate DATE,
    sex CHAR(1) CHECK (sex IN ('М', 'Ж')),
    biography TEXT,
    city VARCHAR(255),
    username VARCHAR(255) NOT NULL,
    password BYTEA NOT NULL,
    UNIQUE (username)
);
