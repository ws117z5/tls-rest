-- --- user_groups: id is the level; add admin flag, drop the Step 1 level col ---
ALTER TABLE IF EXISTS public.user_groups
    ADD COLUMN IF NOT EXISTS is_admin boolean NOT NULL DEFAULT false;
ALTER TABLE IF EXISTS public.user_groups
    DROP COLUMN IF EXISTS access_level;

-- Ensure user_groups.id auto-generates, so groups can be created via the UI.
-- (Legacy schema declared id as a plain integer with no default.)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_attrdef d
        JOIN pg_class c ON c.oid = d.adrelid
        JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = d.adnum
        WHERE c.relname = 'user_groups' AND a.attname = 'id'
    ) THEN
        CREATE SEQUENCE IF NOT EXISTS public.user_groups_id_seq OWNED BY public.user_groups.id;
        PERFORM setval('public.user_groups_id_seq',
                       COALESCE((SELECT MAX(id) FROM public.user_groups), 0) + 1, false);
        ALTER TABLE public.user_groups
            ALTER COLUMN id SET DEFAULT nextval('public.user_groups_id_seq');
    END IF;
END $$;

-- --- users: single group membership ---
ALTER TABLE IF EXISTS public.users
    ADD COLUMN IF NOT EXISTS user_group integer REFERENCES public.user_groups(id);

-- --- Step 1 tables that this revision removes ---
DROP TABLE IF EXISTS public.user_group_members;  -- membership is now users.user_group
DROP TABLE IF EXISTS public.module_rights;        -- replaced by the two tables below

-- --- Rights tables ---
DROP TABLE IF EXISTS public.user_group_rights;
DROP TABLE IF EXISTS public.user_rights;

CREATE TABLE public.user_group_rights (
    id         bigserial   PRIMARY KEY,
    uuid       uuid        NOT NULL DEFAULT uuid_generate_v4(),
    group_id   integer     NOT NULL REFERENCES public.user_groups(id),
    module     text        NOT NULL,
    modes      integer     NOT NULL DEFAULT 0,
    access     integer     NOT NULL DEFAULT 0,
    created    timestamptz NOT NULL DEFAULT now(),
    updated    timestamptz NOT NULL DEFAULT now(),
    created_by integer,
    CONSTRAINT user_group_rights_group_module_key UNIQUE (group_id, module)
);

CREATE TABLE public.user_rights (
    id         bigserial   PRIMARY KEY,
    uuid       uuid        NOT NULL DEFAULT uuid_generate_v4(),
    user_id    integer     NOT NULL REFERENCES public.users(id),
    module     text        NOT NULL,
    modes      integer     NOT NULL DEFAULT 0,
    access     integer     NOT NULL DEFAULT 0,
    created    timestamptz NOT NULL DEFAULT now(),
    updated    timestamptz NOT NULL DEFAULT now(),
    created_by integer,
    CONSTRAINT user_rights_user_module_key UNIQUE (user_id, module)
);

CREATE INDEX IF NOT EXISTS idx_ugr_group ON public.user_group_rights (group_id);
CREATE INDEX IF NOT EXISTS idx_ur_user   ON public.user_rights (user_id);
