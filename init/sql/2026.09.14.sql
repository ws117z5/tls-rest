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
