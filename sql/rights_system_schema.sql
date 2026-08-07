-- Rights System Database Schema
-- This creates the tables matching the rights schema described in the requirements

-- User Tables
CREATE TABLE IF NOT EXISTS "user" (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    login VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE,
    groupid INTEGER REFERENCES usergroup(id),
    active BOOLEAN DEFAULT true,
    currentvisitdatetime TIMESTAMP,
    lastvisitdatetime TIMESTAMP,
    responsibleroleids TEXT -- Comma-separated role IDs
);

-- User-specific module permissions
CREATE TABLE IF NOT EXISTS user_right (
    userid INTEGER REFERENCES "user"(id) ON DELETE CASCADE,
    module VARCHAR(255) NOT NULL,
    permission INTEGER NOT NULL, -- -1=Inherit, 0=Deny, 1=Read, 2=Write
    PRIMARY KEY (userid, module)
);

-- User-specific special permissions  
CREATE TABLE IF NOT EXISTS user_rightspecial (
    userid INTEGER REFERENCES "user"(id) ON DELETE CASCADE,
    module VARCHAR(255) NOT NULL,
    specialid VARCHAR(255) NOT NULL,
    permission INTEGER NOT NULL, -- -1=Inherit, 0=Deny, 1=Read, 2=Write
    PRIMARY KEY (userid, module, specialid)
);

-- User-specific parameters/preferences
CREATE TABLE IF NOT EXISTS user_param (
    userid INTEGER REFERENCES "user"(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    value TEXT,
    PRIMARY KEY (userid, name)
);

-- User-saved filters
CREATE TABLE IF NOT EXISTS user_filter (
    id SERIAL PRIMARY KEY,
    userid INTEGER REFERENCES "user"(id) ON DELETE CASCADE,
    module VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    filter_data JSONB
);

-- User table layout preferences
CREATE TABLE IF NOT EXISTS user_tablelayout (
    userid INTEGER REFERENCES "user"(id) ON DELETE CASCADE,
    module VARCHAR(255) NOT NULL,
    layout_config JSONB,
    PRIMARY KEY (userid, module)
);

-- User table parameters
CREATE TABLE IF NOT EXISTS user_tableparam (
    userid INTEGER REFERENCES "user"(id) ON DELETE CASCADE,
    module VARCHAR(255) NOT NULL,
    param_name VARCHAR(255) NOT NULL,
    param_value TEXT,
    PRIMARY KEY (userid, module, param_name)
);

-- UserGroup Tables
CREATE TABLE IF NOT EXISTS usergroup (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    orderid INTEGER DEFAULT 0
);

-- Group-level module permissions
CREATE TABLE IF NOT EXISTS usergroup_right (
    groupid INTEGER REFERENCES usergroup(id) ON DELETE CASCADE,
    module VARCHAR(255) NOT NULL,
    permission INTEGER NOT NULL, -- -1=Inherit, 0=Deny, 1=Read, 2=Write
    PRIMARY KEY (groupid, module)
);

-- Group-level special permissions
CREATE TABLE IF NOT EXISTS usergroup_rightspecial (
    groupid INTEGER REFERENCES usergroup(id) ON DELETE CASCADE,
    module VARCHAR(255) NOT NULL,
    specialid VARCHAR(255) NOT NULL,
    permission INTEGER NOT NULL, -- -1=Inherit, 0=Deny, 1=Read, 2=Write
    PRIMARY KEY (groupid, module, specialid)
);

-- Related Tables
CREATE TABLE IF NOT EXISTS responsiblerole (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    orderid INTEGER DEFAULT 0
);

-- Sample Data
INSERT INTO usergroup (name, description, orderid) VALUES 
    ('Administrators', 'System administrators with full access', 1),
    ('Editors', 'Content editors with write access to most modules', 2),
    ('Viewers', 'Read-only users', 3)
ON CONFLICT (name) DO NOTHING;

INSERT INTO responsiblerole (name, orderid) VALUES 
    ('Sales', 1),
    ('Customer Service', 2),
    ('Development', 3),
    ('Management', 4)
ON CONFLICT (name) DO NOTHING;

-- Sample permissions (Administrators get write access to all modules)
INSERT INTO usergroup_right (groupid, module, permission) VALUES 
    (1, 'users', 2),    -- Write access to users
    (1, 'posts', 2),    -- Write access to posts
    (2, 'posts', 2),    -- Editors get write access to posts
    (2, 'users', 1),    -- Editors get read access to users
    (3, 'posts', 1),    -- Viewers get read access to posts
    (3, 'users', 0)     -- Viewers denied access to users
ON CONFLICT (groupid, module) DO UPDATE SET permission = EXCLUDED.permission;

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_user_right_userid ON user_right(userid);
CREATE INDEX IF NOT EXISTS idx_user_right_module ON user_right(module);
CREATE INDEX IF NOT EXISTS idx_usergroup_right_groupid ON usergroup_right(groupid);
CREATE INDEX IF NOT EXISTS idx_usergroup_right_module ON usergroup_right(module);
CREATE INDEX IF NOT EXISTS idx_user_groupid ON "user"(groupid);
CREATE INDEX IF NOT EXISTS idx_user_active ON "user"(active);