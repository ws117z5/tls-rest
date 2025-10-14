\c tls-rest

-- SEQUENCE: public.posts_id_seq

-- DROP SEQUENCE IF EXISTS public.posts_id_seq;

CREATE SEQUENCE IF NOT EXISTS public.posts_id_seq
    INCREMENT 1
    START 1
    MINVALUE 1
    MAXVALUE 9223372036854775807
    CACHE 1;

ALTER SEQUENCE public.posts_id_seq
    OWNER TO ws117z5;

-- SEQUENCE: public.users_id_seq

-- DROP SEQUENCE IF EXISTS public.users_id_seq;

CREATE SEQUENCE IF NOT EXISTS public.users_id_seq
    INCREMENT 1
    START 1
    MINVALUE 1
    MAXVALUE 9223372036854775807
    CACHE 1

ALTER SEQUENCE public.users_id_seq
    OWNER TO ws117z5;


-- Trigger: trg_set_updated_at

-- DROP TRIGGER IF EXISTS trg_set_updated_at ON public.users;

CREATE OR REPLACE TRIGGER trg_set_updated_at
    BEFORE UPDATE 
    ON public.users
    FOR EACH ROW
    EXECUTE FUNCTION public.set_updated_at();

-- Table: public.users

-- DROP TABLE IF EXISTS public.users;

CREATE TABLE IF NOT EXISTS public.users
(
    user_name character varying(50) COLLATE pg_catalog."default" NOT NULL,
    email character varying(60) COLLATE pg_catalog."default",
    auth_parent_hash text COLLATE pg_catalog."default",
    auth_hash text COLLATE pg_catalog."default",
    auth_by smallint,
    auth_session_expires timestamp with time zone,
    uuid uuid NOT NULL DEFAULT uuid_generate_v4(),
    id integer DEFAULT nextval('users_id_seq'::regclass),
    first_name character varying(50) COLLATE pg_catalog."default" NOT NULL,
    last_name character varying(60) COLLATE pg_catalog."default",
    image text COLLATE pg_catalog."default",
    created_at timestamp without time zone NOT NULL DEFAULT now(),
    updated_at timestamp without time zone NOT NULL DEFAULT now(),
    CONSTRAINT users_pkey PRIMARY KEY (uuid),
    CONSTRAINT users_id_key UNIQUE (id)
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS public.users
    OWNER to ws117z5;

-- Table: public.posts

-- DROP TABLE IF EXISTS public.posts;

CREATE TABLE IF NOT EXISTS public.posts
(
    uuid uuid NOT NULL DEFAULT uuid_generate_v4(),
    id integer NOT NULL DEFAULT nextval('posts_id_seq'::regclass),
    title text COLLATE pg_catalog."default" NOT NULL,
    text text COLLATE pg_catalog."default",
    drive_url text COLLATE pg_catalog."default",
    drive_id character varying(40) COLLATE pg_catalog."default",
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    CONSTRAINT posts_pkey PRIMARY KEY (uuid)
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS public.posts
    OWNER to ws117z5;

-- Trigger: trg_set_updated_at

-- DROP TRIGGER IF EXISTS trg_set_updated_at ON public.posts;

CREATE OR REPLACE TRIGGER trg_set_updated_at
    BEFORE UPDATE 
    ON public.posts
    FOR EACH ROW
    EXECUTE FUNCTION public.set_updated_at();