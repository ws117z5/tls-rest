-- 2026.09.13 — messages: direct messages between two users (sender_id,
-- recipient_id), unlike comments' polymorphic (module_id, row_id) target. The
-- engine also auto-creates these columns from the messages module fieldset;
-- this file pins the types and adds the lookup indexes.
CREATE TABLE IF NOT EXISTS public.messages (
    id           bigserial   PRIMARY KEY,
    uuid         uuid        NOT NULL DEFAULT uuid_generate_v4(),
    sender_id    integer     NOT NULL,
    recipient_id integer     NOT NULL,
    body         text        NOT NULL,
    read_at      timestamptz,
    created_by   integer,
    created      timestamptz NOT NULL DEFAULT now(),
    updated      timestamptz NOT NULL DEFAULT now(),
    access       integer     NOT NULL DEFAULT 0
);

-- Thread lookup (either direction) and inbox unread-count queries.
CREATE INDEX IF NOT EXISTS idx_messages_sender_recipient ON public.messages (sender_id, recipient_id, created);
CREATE INDEX IF NOT EXISTS idx_messages_recipient_unread ON public.messages (recipient_id, sender_id) WHERE read_at IS NULL;

INSERT INTO public.translations (key, locale, value) VALUES
('Message', 'ru', 'Сообщение'),
('Messages', 'ru', 'Сообщения'),
('Send', 'ru', 'Отправить'),
('Write a message…', 'ru', 'Напишите сообщение…'),
('No messages yet.', 'ru', 'Пока нет сообщений.'),
('Could not load messages.', 'ru', 'Не удалось загрузить сообщения.'),
('Could not send message.', 'ru', 'Не удалось отправить сообщение.'),
('Sign in to send a message.', 'ru', 'Войдите, чтобы отправить сообщение.'),
('No conversations yet.', 'ru', 'Пока нет переписок.'),
('View profile', 'ru', 'Открыть профиль'),
('Member since', 'ru', 'Участник с'),
('Profile not found.', 'ru', 'Профиль не найден.'),
('Sign in to view profiles.', 'ru', 'Войдите, чтобы просматривать профили.')
ON CONFLICT (key, locale) DO NOTHING;

-- friends: one row per relationship between two users — "pending" from
-- requester_id to recipient_id, or "accepted" once the recipient accepts.
-- Either side removing it (unfriend/cancel/decline) just deletes the row.
-- The engine also auto-creates these columns from the friend_requests module
-- fieldset; this file pins the types and adds the pair-uniqueness index.
CREATE TABLE IF NOT EXISTS public.friends (
    id           bigserial   PRIMARY KEY,
    uuid         uuid        NOT NULL DEFAULT uuid_generate_v4(),
    requester_id integer     NOT NULL,
    recipient_id integer     NOT NULL,
    status       varchar(20) NOT NULL DEFAULT 'pending',
    created_by   integer,
    created      timestamptz NOT NULL DEFAULT now(),
    updated      timestamptz NOT NULL DEFAULT now(),
    access       integer     NOT NULL DEFAULT 0,
    CONSTRAINT friends_status_check CHECK (status IN ('pending', 'accepted')),
    CONSTRAINT friends_not_self CHECK (requester_id <> recipient_id)
);

-- One relationship per pair regardless of direction.
CREATE UNIQUE INDEX IF NOT EXISTS idx_friends_pair
    ON public.friends (LEAST(requester_id, recipient_id), GREATEST(requester_id, recipient_id));
CREATE INDEX IF NOT EXISTS idx_friends_recipient ON public.friends (recipient_id, status);

INSERT INTO public.translations (key, locale, value) VALUES
('Add Friend', 'ru', 'Добавить в друзья'),
('Cancel Request', 'ru', 'Отменить запрос'),
('Accept', 'ru', 'Принять'),
('Decline', 'ru', 'Отклонить'),
('Friends', 'ru', 'Друзья'),
('Remove Friend', 'ru', 'Удалить из друзей'),
('Request sent', 'ru', 'Запрос отправлен'),
('wants to be friends', 'ru', 'хочет добавить вас в друзья')
ON CONFLICT (key, locale) DO NOTHING;

-- posts: replace the inert "public" checkbox with a sharing-list ACL —
-- visible_users/visible_groups (jsonb arrays of ids), enforced by the engine's
-- VisibilityUsersField/VisibilityGroupsField (see posts.go). A post shared
-- with nobody (both empty, the default for every existing row too, since a
-- NULL/absent array has no elements) is visible only to its author and
-- admins — a stricter default than before, by design.
ALTER TABLE IF EXISTS public.posts ADD COLUMN IF NOT EXISTS visible_users jsonb;
ALTER TABLE IF EXISTS public.posts ADD COLUMN IF NOT EXISTS visible_groups jsonb;
ALTER TABLE IF EXISTS public.posts DROP COLUMN IF EXISTS public;

INSERT INTO public.translations (key, locale, value) VALUES
('Visible To (Users)', 'ru', 'Видно пользователям'),
('Specific users who may also view this post, besides you and admins', 'ru', 'Отдельные пользователи, которым также видна эта запись, помимо вас и администраторов'),
('Visible To (Groups)', 'ru', 'Видно группам'),
('User groups who may also view this post, besides you and admins', 'ru', 'Группы пользователей, которым также видна эта запись, помимо вас и администраторов'),
('User', 'ru', 'Пользователь'),
('Group', 'ru', 'Группа')
ON CONFLICT (key, locale) DO NOTHING;

-- Seed the three-tier group model used by the integration tests
-- (test/rights_test.go): 0 = admin (is_admin, bypasses every check), 1 =
-- guest (see auth.GuestGroupID — what an anonymous caller resolves as), 2 =
-- users (ordinary signed-in members). Note this makes id ASCENDING ==
-- privilege DESCENDING for admin specifically, unlike the rest of the group
-- system (id = access level, higher = more) — harmless here because is_admin
-- is an independent flag and ResolveIsAdmin/ResolveUserAccessLevel already
-- treat admin status and access level as separate axes; admins bypass the
-- level check entirely (see fillSessionRights / CanAccessRow).
--
-- The actual user accounts (an admin and an ordinary "users" member) are NOT
-- seeded here on purpose: test/rights_test.go creates a fresh, uniquely
-- emailed account for each of the two roles at the start of every run and
-- deletes them again at the end (see TestMain / createFixtureUser), so the
-- suite is self-contained on any DB that already has this migration applied
-- — this file only needs to own the durable role CONFIGURATION (the groups
-- and their rights), not throwaway per-run test data.
INSERT INTO public.user_groups (id, name, is_admin) VALUES
(0, 'admin', true),
(1, 'guest', false),
(2, 'users', false)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, is_admin = EXCLUDED.is_admin;

-- Keep user_groups' id sequence ahead of these hand-picked low ids, so the
-- next group created through the admin UI doesn't collide with them.
SELECT setval('public.user_groups_id_seq', GREATEST(2, (SELECT MAX(id) FROM public.user_groups)));

-- Baseline module rights. Nothing is needed for group 0 (admin) — is_admin
-- bypasses every mode/row check regardless of these tables. "posts" already
-- defaults every module default to READ (list+view) for everyone
-- (DefaultPermission in posts.go), so these rows are what actually
-- DIFFERENTIATE guest from a signed-in "users" member: guests stay read-only,
-- ordinary users can also create and edit (their own — see posts'
-- VisibilityUsersField/VisibilityGroupsField ACL for which rows they can
-- reach at all).
INSERT INTO public.user_group_rights (group_id, module, modes) VALUES
(1, 'posts', 3),   -- guest: list(1) + view(2)
(2, 'posts', 15)   -- users: list(1) + view(2) + create(4) + edit(8)
ON CONFLICT (group_id, module) DO UPDATE SET modes = EXCLUDED.modes;

-- Per-field rights (user_group_rights.fields — see the "Field Access" table on
-- Group Rights / modulerights.go's fieldsField): a JSON string of
-- {"<field>": ["list","view","create","edit","delete"]}. A module with ANY
-- row here for a group becomes FIELD-RESTRICTED for that group — every field
-- not listed is hidden from schema and stripped from returned data (see
-- accessfilter.go's fieldVisibleInSchema/fieldReadableInData) — so this also
-- exercises that layer, not just the coarser list/view/create/edit/delete
-- modes above. Guests get title+author only on "posts"; group 2 ("users")
-- is deliberately left unrestricted (no row) for contrast.
UPDATE public.user_group_rights
SET fields = '{"title": ["list", "view"], "author": ["list", "view"]}'
WHERE group_id = 1 AND module = 'posts';

-- The post shared with both groups for TestPosts_FieldRights (comparing what
-- guest vs. an unrestricted role gets back from the same row) is also
-- created and torn down by the Go test now, not seeded here — same reasoning
-- as the user accounts above: it needs a real created_by pointing at
-- whichever admin id that test run's own fixture got, which this file has no
-- way to know in advance.
