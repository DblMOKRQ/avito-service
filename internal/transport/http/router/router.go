package router

import (
	"go.uber.org/zap"

	"avito/internal/transport/http/handler"
	"avito/internal/transport/http/middleware"

	"github.com/gin-gonic/gin"
)

type Router struct {
	rout *gin.Engine
	h    *handler.Handler
	log  *zap.Logger
}

func NewRouter(h *handler.Handler, mode string, log *zap.Logger) *Router {
	switch mode {
	case "debug":
		gin.SetMode(gin.DebugMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}
	router := &Router{
		rout: gin.Default(),
		h:    h,
		log:  log.Named("router"),
	}
	router.setupRouter()

	return router
}

func (r *Router) setupRouter() {
	r.rout.Use(middleware.LoggingMiddleware(r.log))
	gr := r.rout.Group("")

	gr.GET("/stats", r.h.GetStats)
	r.addUsers(gr)
	r.addTeam(gr)
	r.addPR(gr)

}

func (r *Router) addUsers(rg *gin.RouterGroup) {
	users := rg.Group("/users")

	users.POST("/setIsActive", r.h.SetUserActiveStatus)
	users.GET("/getReview", r.h.GetUserReview)

}

func (r *Router) addTeam(rg *gin.RouterGroup) {
	team := rg.Group("/team")

	team.POST("/add", r.h.CreateTeam)
	team.GET("/get", r.h.GetTeam)
}

func (r *Router) addPR(rg *gin.RouterGroup) {
	pullRequest := rg.Group("/pullRequest")

	pullRequest.POST("/create", r.h.CreatePR)
	pullRequest.POST("/merge", r.h.SetMerge)
	pullRequest.POST("/reassign", r.h.Reassign)
}

func (r *Router) GetEngine() *gin.Engine {
	return r.rout
}

func (r *Router) Start(addr string) error {
	return r.rout.Run(addr)
}
