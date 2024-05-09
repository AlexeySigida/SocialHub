CREATE TABLE public.users (
    id SERIAL PRIMARY KEY,
    "name" VARCHAR(255),
    surname VARCHAR(255),
    birthdate DATE,
    sex CHAR(1) CHECK (sex IN ('М', 'Ж')),
    interest TEXT,
    city VARCHAR(255)
);
