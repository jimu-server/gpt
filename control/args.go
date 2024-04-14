package control

type CreateConversationArgs struct {
	Title string `json:"title" form:"title" binding:"required"`
}

type DelConversationArgs struct {
	Id string `json:"id" form:"id" binding:"required"`
}
