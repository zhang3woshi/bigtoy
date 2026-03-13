package controllers

import (
	"net/http"

	"github.com/beego/beego/v2/server/web"
)

type PageController struct {
	web.Controller
}

func (c *PageController) Public() {
	c.TplName = "index.html"
}

func (c *PageController) Detail() {
	c.TplName = "model.html"
}

func (c *PageController) Admin() {
	if _, ok := sessionFromRequest(c.Ctx.Request); !ok {
		c.Redirect("/login.html", http.StatusFound)
		return
	}
	c.TplName = "admin.html"
}

func (c *PageController) AdminEdit() {
	if _, ok := sessionFromRequest(c.Ctx.Request); !ok {
		c.Redirect("/login.html", http.StatusFound)
		return
	}
	c.TplName = "admin-edit.html"
}

func (c *PageController) Login() {
	if _, ok := sessionFromRequest(c.Ctx.Request); ok {
		c.Redirect("/admin.html", http.StatusFound)
		return
	}
	c.TplName = "login.html"
}
