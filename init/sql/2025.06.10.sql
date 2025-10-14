-- SEQUENCE: public.modules_id_seq

-- DROP SEQUENCE IF EXISTS public.modules_id_seq;

CREATE SEQUENCE IF NOT EXISTS public.modules_id_seq
    INCREMENT 1
    START 1
    MINVALUE 1
    MAXVALUE 2147483647
    CACHE 1;

ALTER SEQUENCE public.modules_id_seq
    OWNED BY public.modules.id;

ALTER SEQUENCE public.modules_id_seq
    OWNER TO ws117z5;

-- Table: public.modules

-- DROP TABLE IF EXISTS public.modules;

CREATE TABLE IF NOT EXISTS public.modules
(
    id integer NOT NULL DEFAULT nextval('modules_id_seq'::regclass),
    uuid uuid NOT NULL DEFAULT uuid_generate_v4(),
    name character varying(40) COLLATE pg_catalog."default" NOT NULL,
    CONSTRAINT id PRIMARY KEY (id)
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS public.modules
    OWNER to ws117z5;

-- Table: public.user_rights

-- DROP TABLE IF EXISTS public.user_rights;

CREATE TABLE IF NOT EXISTS public.user_rights
(
    user_id integer NOT NULL,
    "right" smallint NOT NULL DEFAULT 0,
    module_id integer NOT NULL,
    CONSTRAINT module_id FOREIGN KEY (module_id)
        REFERENCES public.modules (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        NOT VALID,
    CONSTRAINT user_id FOREIGN KEY (user_id)
        REFERENCES public.users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE NO ACTION
        NOT VALID
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS public.user_rights
    OWNER to ws117z5;

-- Table: public.user_groups

-- DROP TABLE IF EXISTS public.user_groups;

CREATE TABLE IF NOT EXISTS public.user_groups
(
    id integer NOT NULL,
    name character varying(40) COLLATE pg_catalog."default",
    CONSTRAINT user_groups_pkey PRIMARY KEY (id)
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS public.user_groups
    OWNER to ws117z5;

-- Table: public.user_group_rights

-- DROP TABLE IF EXISTS public.user_group_rights;

CREATE TABLE IF NOT EXISTS public.user_group_rights
(
    group_id integer NOT NULL,
    module_id integer NOT NULL,
    "right" smallint
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS public.user_group_rights
    OWNER to ws117z5;