INSERT INTO public.translations (key, locale, value) VALUES
('Execution log', 'ru', 'Журнал выполнения')
ON CONFLICT (key, locale) DO NOTHING;
