package control

type CreateConversationArgs struct {
	Title string `json:"title" form:"title" binding:"required"`
}
