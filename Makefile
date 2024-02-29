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