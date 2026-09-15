-- access_log: country resolved from the client IP at write time (see
-- accesslog.CountryForIP) — a 2-letter ISO code, or NULL until an admin runs
-- the "Load GeoIP tables" action from the Actions page (LoadGeoIP).
ALTER TABLE IF EXISTS public.access_log ADD COLUMN IF NOT EXISTS country varchar(2);

-- session_id already exists on most installs (legacy column); ensured here
-- for a fresh one, now that accesslog.persist writes it (see Entry.SessionID).
-- Sized 512 to hold the IP+UserAgent fallback middleware.getSessionID uses
-- when a request carries no session cookie/header.
ALTER TABLE IF EXISTS public.access_log ADD COLUMN IF NOT EXISTS session_id varchar(512);
ALTER TABLE IF EXISTS public.access_log ALTER COLUMN session_id TYPE varchar(512);

INSERT INTO public.translations (key, locale, value) VALUES
('Statistics', 'ru', 'Статистика'),
('Module', 'ru', 'Модуль'),
('Page', 'ru', 'Страница'),
('Country', 'ru', 'Страна'),
('All', 'ru', 'Все'),
('From', 'ru', 'С'),
('To', 'ru', 'По'),
('Apply', 'ru', 'Применить'),
('Total requests', 'ru', 'Всего запросов'),
('Blocked requests', 'ru', 'Заблокировано запросов'),
('New users', 'ru', 'Новых пользователей'),
('Only the time period applies to this count — signups aren''t tied to a module, page, country, IP, or user agent.', 'ru', 'Учитывается только период — регистрации не привязаны к модулю, странице, стране, IP или user agent.'),
('By module', 'ru', 'По модулям'),
('By page', 'ru', 'По страницам'),
('By country', 'ru', 'По странам'),
('Count', 'ru', 'Количество'),
('No data for this range.', 'ru', 'Нет данных за этот период.'),
('Live', 'ru', 'В реальном времени'),
('Requests', 'ru', 'Запросы'),
('Requests (module)', 'ru', 'Запросы (модуль)'),
('DB queries', 'ru', 'Запросы к БД'),
('CPU load', 'ru', 'Нагрузка CPU'),
('Memory', 'ru', 'Память'),
('IP', 'ru', 'IP'),
('User agent', 'ru', 'User agent'),
('Period', 'ru', 'Период'),
('Past day', 'ru', 'За день'),
('Past week', 'ru', 'За неделю'),
('Past month', 'ru', 'За месяц'),
('Past year', 'ru', 'За год'),
('Ever', 'ru', 'За всё время'),
('View', 'ru', 'Вид'),
('Table', 'ru', 'Таблица'),
('Bar', 'ru', 'Столбцы'),
('Pie', 'ru', 'Круговая'),
('By IP', 'ru', 'По IP'),
('By user agent', 'ru', 'По User agent'),
('Unique sessions', 'ru', 'Уникальных сессий'),
('Memory breakdown', 'ru', 'Разбивка памяти'),
('App heap', 'ru', 'Память приложения (heap)'),
('Engine overhead', 'ru', 'Накладные расходы движка'),
('Database size (disk)', 'ru', 'Размер базы данных (диск)'),
('Session', 'ru', 'Сессия'),
('Load GeoIP', 'ru', 'Загрузить GeoIP'),
('GeoIP loaded', 'ru', 'GeoIP загружен'),
('GeoIP not loaded', 'ru', 'GeoIP не загружен'),
('Resolved', 'ru', 'Определено'),
('countries', 'ru', 'стран'),
('Loaded', 'ru', 'Загружено'),
('ranges', 'ru', 'диапазонов'),
('GeoIP not loaded — run it from Actions', 'ru', 'GeoIP не загружен — запустите на странице Actions'),
('Actions', 'ru', 'Действия'),
('No actions registered.', 'ru', 'Нет зарегистрированных действий.'),
('Run now', 'ru', 'Запустить'),
('Repeat every (seconds, 0 = off)', 'ru', 'Повторять каждые (секунд, 0 = выкл)'),
('Save', 'ru', 'Сохранить'),
('Currently every', 'ru', 'Сейчас каждые'),
('Last run', 'ru', 'Последний запуск')
ON CONFLICT (key, locale) DO NOTHING;

-- Explicit rights rows for every group on every module missing one (posts and
-- admins/access_log already exist and are skipped).
INSERT INTO user_group_rights (group_id, module, modes, fields)
SELECT v.group_id, v.module, v.modes, v.fields
FROM (VALUES
  (1,'users',0,'{}'), (1,'words',0,'{}'), (1,'user_groups',0,'{}'),
  (1,'user_group_rights',0,'{}'), (1,'user_rights',0,'{}'),
  (1,'access_log',0,'{}'), (1,'access_rule',0,'{}'),
  (1,'images',3,'{"field":["list","view"],"filename":["list","view"],"mime_type":["list","view"],"module":["list","view"],"preview":["list","view"],"record_id":["list","view"]}'),
  (1,'comments',0,'{}'), (1,'likes',0,'{}'), (1,'message_log',0,'{}'),
  (1,'friend_requests',0,'{}'), (1,'contact_messages',0,'{}'),
  (1,'deletion_requests',0,'{}'), (1,'translations',0,'{}'), (1,'config',0,'{}'),
  (1,'papers',31,'{"deleted":["list","view","create","edit","delete"],"has_password":["list","view","create","edit","delete"],"hash":["list","view","create","edit","delete"],"name":["list","view","create","edit","delete"],"password":["list","view","create","edit","delete"],"timer":["list","view","create","edit","delete"],"users":["list","view","create","edit","delete"],"wrong_answers":["list","view","create","edit","delete"]}'),
  (2,'users',0,'{}'), (2,'words',0,'{}'), (2,'user_groups',0,'{}'),
  (2,'user_group_rights',0,'{}'), (2,'user_rights',0,'{}'),
  (2,'access_log',0,'{}'), (2,'access_rule',0,'{}'),
  (2,'images',3,'{"field":["list","view"],"filename":["list","view"],"mime_type":["list","view"],"module":["list","view"],"preview":["list","view"],"record_id":["list","view"]}'),
  (2,'comments',0,'{}'), (2,'likes',0,'{}'), (2,'message_log',0,'{}'),
  (2,'friend_requests',0,'{}'), (2,'contact_messages',0,'{}'),
  (2,'deletion_requests',0,'{}'), (2,'translations',0,'{}'), (2,'config',0,'{}'),
  (2,'papers',31,'{"deleted":["list","view","create","edit","delete"],"has_password":["list","view","create","edit","delete"],"hash":["list","view","create","edit","delete"],"name":["list","view","create","edit","delete"],"password":["list","view","create","edit","delete"],"timer":["list","view","create","edit","delete"],"users":["list","view","create","edit","delete"],"wrong_answers":["list","view","create","edit","delete"]}'),
  (0,'users',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"email":["list","view","create","edit","delete"],"first_name":["list","view","create","edit","delete"],"group":["list","view","create","edit","delete"],"groups":["list","view","create","edit","delete"],"image":["list","view","create","edit","delete"],"last_name":["list","view","create","edit","delete"],"user_name":["list","view","create","edit","delete"]}'),
  (0,'posts',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"author":["list","view","create","edit","delete"],"content":["list","view","create","edit","delete"],"group":["list","view","create","edit","delete"],"images":["list","view","create","edit","delete"],"title":["list","view","create","edit","delete"],"user":["list","view","create","edit","delete"],"visible_groups":["list","view","create","edit","delete"],"visible_users":["list","view","create","edit","delete"]}'),
  (0,'words',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"fail":["list","view","create","edit","delete"],"success":["list","view","create","edit","delete"],"translations":["list","view","create","edit","delete"],"tries":["list","view","create","edit","delete"],"win_rate":["list","view","create","edit","delete"],"word":["list","view","create","edit","delete"]}'),
  (0,'user_groups',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"active":["list","view","create","edit","delete"],"description":["list","view","create","edit","delete"],"is_admin":["list","view","create","edit","delete"],"name":["list","view","create","edit","delete"]}'),
  (0,'user_group_rights',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"group_id":["list","view","create","edit","delete"],"module":["list","view","create","edit","delete"],"modes":["list","view","create","edit","delete"],"fields":["list","view","create","edit","delete"]}'),
  (0,'user_rights',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"user_id":["list","view","create","edit","delete"],"module":["list","view","create","edit","delete"],"modes":["list","view","create","edit","delete"],"fields":["list","view","create","edit","delete"]}'),
  (0,'access_rule',31,'{"action":["list","view","create","edit","delete"],"cidr":["list","view","create","edit","delete"],"enabled":["list","view","create","edit","delete"],"firewall":["list","view","create","edit","delete"],"note":["list","view","create","edit","delete"],"priority":["list","view","create","edit","delete"],"user_agent":["list","view","create","edit","delete"]}'),
  (0,'images',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"field":["list","view","create","edit","delete"],"filename":["list","view","create","edit","delete"],"mime_type":["list","view","create","edit","delete"],"module":["list","view","create","edit","delete"],"preview":["list","view","create","edit","delete"],"record_id":["list","view","create","edit","delete"]}'),
  (0,'comments',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"body":["list","view","create","edit","delete"],"module_id":["list","view","create","edit","delete"],"row_id":["list","view","create","edit","delete"]}'),
  (0,'likes',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"module_id":["list","view","create","edit","delete"],"row_id":["list","view","create","edit","delete"],"user_id":["list","view","create","edit","delete"],"value":["list","view","create","edit","delete"]}'),
  (0,'message_log',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"body":["list","view","create","edit","delete"],"recipient_id":["list","view","create","edit","delete"],"sender_id":["list","view","create","edit","delete"]}'),
  (0,'friend_requests',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"recipient_id":["list","view","create","edit","delete"],"requester_id":["list","view","create","edit","delete"],"status":["list","view","create","edit","delete"]}'),
  (0,'contact_messages',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"email":["list","view","create","edit","delete"],"handled":["list","view","create","edit","delete"],"message":["list","view","create","edit","delete"],"name":["list","view","create","edit","delete"],"subject":["list","view","create","edit","delete"]}'),
  (0,'deletion_requests',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"email":["list","view","create","edit","delete"],"handled":["list","view","create","edit","delete"],"note":["list","view","create","edit","delete"],"reason":["list","view","create","edit","delete"],"user_id":["list","view","create","edit","delete"]}'),
  (0,'translations',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"key":["list","view","create","edit","delete"],"locale":["list","view","create","edit","delete"],"value":["list","view","create","edit","delete"]}'),
  (0,'config',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"date_format":["list","view","create","edit","delete"],"scope_id":["list","view","create","edit","delete"],"scope":["list","view","create","edit","delete"],"theme":["list","view","create","edit","delete"]}'),
  (0,'papers',31,'{"uuid":["list","view","create","edit","delete"],"created":["list","view","create","edit","delete"],"updated":["list","view","create","edit","delete"],"created_by":["list","view","create","edit","delete"],"access":["list","view","create","edit","delete"],"deleted":["list","view","create","edit","delete"],"has_password":["list","view","create","edit","delete"],"hash":["list","view","create","edit","delete"],"name":["list","view","create","edit","delete"],"password":["list","view","create","edit","delete"],"timer":["list","view","create","edit","delete"],"users":["list","view","create","edit","delete"],"wrong_answers":["list","view","create","edit","delete"]}')
) AS v(group_id, module, modes, fields)
WHERE NOT EXISTS (
  SELECT 1 FROM user_group_rights ugr
  WHERE ugr.group_id = v.group_id AND ugr.module = v.module
);

-- profile page grant: pages are now rights-gated like modules (see PageAbstract.hasMode).
INSERT INTO user_group_rights (group_id, module, modes, fields)
SELECT 2, 'profile', 10, '{}'
WHERE NOT EXISTS (
  SELECT 1 FROM user_group_rights WHERE group_id = 2 AND module = 'profile'
);

-- guests/posts predates this file with fields left NULL (unrestricted); fixing in place.
UPDATE user_group_rights
SET fields = '{"title":["list","view"],"author":["list","view"]}'
WHERE group_id = 1 AND module = 'posts' AND fields IS NULL;
