CREATE DATABASE "tls-rest"
    WITH 
    OWNER = ws117z5
    ENCODING = 'UTF8'
    CONNECTION LIMIT = -1;

COMMENT ON DATABASE "tls-rest"
    IS 'testing stuff';


CREATE TABLE public.users
(
    rights integer NOT NULL,
    name text COLLATE pg_catalog."default" NOT NULL,
    id uuid NOT NULL,
    email text COLLATE pg_catalog."default",
    edited timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created timestamp with time zone NOT NULL,
    auth_parent_hash text COLLATE pg_catalog."default",
    auth_hash text COLLATE pg_catalog."default",
    auth_by smallint,
    auth_session_expires timestamp with time zone,
    CONSTRAINT users_pkey PRIMARY KEY (id)
)
WITH (
    OIDS = FALSE
)
TABLESPACE pg_default;

ALTER TABLE public.users
    OWNER to ws117z5;