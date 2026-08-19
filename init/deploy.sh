export $(grep -v '^#' ../.private/.env | xargs)

#push sql dump
pg_dump -U $PG_USER -d $PG_NAME | ssh $PG_USER@$WORKSTATION_ADDR "psql -U $WORKSTATION_PG_USER -d $WORKSTATION_PG_NAME"

#push .env
scp ../.private/.env.workstation $PG_USER@$WORKSTATION_ADDR:/opt/workstation/.env
#push keys
scp ../.private/cloudflare/cert.pem $PG_USER@$WORKSTATION_ADDR:/opt/workstation/certs/cert.pem
scp ../.private/cloudflare/key.pem $PG_USER@$WORKSTATION_ADDR:/opt/workstation/certs/key.pem