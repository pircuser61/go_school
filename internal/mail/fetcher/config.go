package fetcher

type Config struct {
	ImapConnection string `yaml:"imap_connection"`
	ImapUserName   string `yaml:"imap_user_name"`
	ImapPassword   string `yaml:"imap_password"`
	ImapMailBox    string `yaml:"imap_mail_box"`
}
