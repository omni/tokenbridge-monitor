CREATE ROLE readonly;
GRANT CONNECT ON DATABASE db TO readonly;
GRANT USAGE ON SCHEMA public TO readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly;

CREATE USER read_user WITH PASSWORD 'read_user_pass';
GRANT readonly TO read_user;
