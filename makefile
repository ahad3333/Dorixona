migration-up:
	migrate -path ./migrations/postgres -database 'postgresql://postgres:LVoaaZpQnLHpnMDriIKOqrOLCiAbLLWF@yamabiko.proxy.rlwy.net:15284/railway' up

migration-down:
	migrate -path ./migrations/postgres -database 'postgresql://postgres:LVoaaZpQnLHpnMDriIKOqrOLCiAbLLWF@yamabiko.proxy.rlwy.net:15284/railway' down