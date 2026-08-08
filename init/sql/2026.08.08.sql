CREATE TABLE IF NOT EXISTS prooms (
    id          BIGSERIAL     PRIMARY KEY,               
    uuid        uuid NOT NULL DEFAULT uuid_generate_v4(),      
    name        TEXT          NOT NULL,                  
    password    TEXT,                                     
    created_by  TEXT,                                    
    users       TEXT          NOT NULL DEFAULT '[]',      
    created     TIMESTAMPTZ   NOT NULL DEFAULT now()     
);
 
CREATE INDEX IF NOT EXISTS prooms_created_idx ON prooms (created);