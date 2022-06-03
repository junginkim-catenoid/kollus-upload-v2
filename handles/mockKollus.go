package handles

type oAuthMessage struct {
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	AccessToken string `json:"access_token"`
}

type categoryMessage struct {
	Data categoryData
}

type categoryData struct {
	Id         int
	key        string
	Name       string
	Level_Path string
}

type preHookMessage struct {
	Error  int
	Result preHookResult
}

type preHookResult struct {
	Target string
	Local  preHookResultMessage
}

type preHookResultMessage struct {
	Path     string
	Filename string
}

type endHookMessage struct {
	Error  int
	Result endHookResult
}

type endHookResult struct {
	Content_type string
	Body         string
}
