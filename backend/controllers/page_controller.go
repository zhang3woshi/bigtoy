package controllers

import (
	"net/http"
	"os"
	"strings"

	"github.com/beego/beego/v2/server/web"
)

type PageController struct {
	web.Controller
}

const defaultSiteFilingURL = "https://beian.miit.gov.cn"

func (c *PageController) Public() {
	if c.Data == nil {
		c.Data = map[interface{}]interface{}{}
	}
	siteFilingText, siteFilingURL := resolveSiteFilingConfig()
	c.Data["SiteFilingText"] = siteFilingText
	c.Data["SiteFilingURL"] = siteFilingURL
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

func resolveSiteFilingConfig() (string, string) {
	siteFilingText := strings.TrimSpace(os.Getenv("BIGTOY_SITE_FILING_TEXT"))
	if siteFilingText == "" {
		return "", ""
	}

	siteFilingURL := strings.TrimSpace(os.Getenv("BIGTOY_SITE_FILING_URL"))
	if siteFilingURL == "" {
		siteFilingURL = defaultSiteFilingURL
	}

	return siteFilingText, siteFilingURL
}
