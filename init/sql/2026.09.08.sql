-- 2026.09.08 — users group membership: move from a single primary group
-- (users.user_group) + a join table (user_group_members) to one jsonb array of
-- group ids on the user row (users.groups = [12, 3]).
--
-- Apply in two stages:
--   PART 1  — additive + backfill. Safe to run now; the old columns keep working.
--   PART 2  — destructive. Run ONLY after the deployed code no longer reads
--             users.user_group or user_group_members, i.e. after these are
--             updated: engine/controllers/auth/resolve.go (userGroupsSubquery),
--             engine/controllers/config/config.go, modules/posts/filters.go,
--             engine/modules/users/users.go (the "groups" TableData query).

-- ============================ PART 1: additive =============================

ALTER TABLE IF EXISTS public.users
    ADD COLUMN IF NOT EXISTS groups jsonb NOT NULL DEFAULT '[]'::jsonb;

-- Backfill: union of the primary group and any user_group_members rows, as a
-- sorted jsonb array of ints. Only touches users still at the default, so it is
-- safe to re-run. The user_group_members half is skipped if that table was
-- already dropped by an earlier revision.
DO $$
BEGIN
    IF to_regclass('public.user_group_members') IS NOT NULL THEN
        UPDATE public.users u
        SET groups = COALESCE((
                SELECT jsonb_agg(DISTINCT gid ORDER BY gid)
                FROM (
                    SELECT u.user_group AS gid WHERE u.user_group IS NOT NULL
                    UNION
                    SELECT m.group_id FROM public.user_group_members m WHERE m.user_id = u.id
                ) s
            ), '[]'::jsonb)
        WHERE u.groups IS NULL OR u.groups = '[]'::jsonb;
    ELSE
        UPDATE public.users u
        SET groups = to_jsonb(ARRAY[u.user_group])
        WHERE (u.groups IS NULL OR u.groups = '[]'::jsonb) AND u.user_group IS NOT NULL;
    END IF;
END $$;

-- Containment index for "which groups is this user in" lookups (groups @> '[5]').
CREATE INDEX IF NOT EXISTS idx_users_groups ON public.users USING gin (groups);

-- ===================== PART 2: destructive ================================
-- The code that read users.user_group / user_group_members is updated in this
-- same change (auth/resolve.go, config/config.go, posts/filters.go, users.go),
-- so these drops are safe once the new binary is deployed. If you apply
-- migrations before deploying, run PART 1 now and PART 2 after the deploy.

ALTER TABLE IF EXISTS public.users DROP COLUMN IF EXISTS user_group;
DROP TABLE IF EXISTS public.user_group_members;
