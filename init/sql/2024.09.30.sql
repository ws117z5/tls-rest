ALTER TABLE IF EXISTS public.users
    ADD COLUMN "uuid" uuid DEFAULT uuid_generate_v4();