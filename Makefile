# для запуска этой команды нужен форк gci https://github.com/amidgo/gci
format-imports:
	gci write \
	 --custom-order \
	 -s standard \
	 -s module \
	 -s blank \
	 -s "module_prefix(gitlab.services.mts.ru/)" \
	 -s "prefix(gitlab.services.mts.ru/jocasta/pipeliner/)" \
		internal

pipeliner:
	ACCESS_KEY=AABBCCDD \
	DB_PASS=4V7Debjee4s2KvTg \
	DB_USER=jocasta \
	IMAP_PASSWORD=5teo26jBmj \
	SECRET_ACCESS_KEY=aabbccddeeffgghhii \
	SSO_SECRET=QgsBq2FmgWGCq1o6t9aU4JFDP2nT2FCN \
	go run cmd/pipeliner/main.go