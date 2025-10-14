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
    OWNED BY users.id;

ALTER SEQUENCE public.users_id_seq
    OWNER TO ws117z5;

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
    CONSTRAINT posts_pkey PRIMARY KEY (uuid)
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS public.posts
    OWNER to ws117z5;