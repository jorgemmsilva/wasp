package webapi

import (
	"net/url"

	"github.com/labstack/echo/v4"
)

// Middleware to unescape any supplied path (/path/foo%40bar/) parameter
// Query parameters (?name=foo%40bar) get unescaped by default.
func middlewareUnescapePath(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		escapedPathParams := c.ParamValues()
		unescapedPathParams := make([]string, len(escapedPathParams))

		for i, param := range escapedPathParams {
			unescapedParam, err := url.PathUnescape(param)

			if err != nil {
				unescapedPathParams[i] = param
			} else {
				unescapedPathParams[i] = unescapedParam
			}
		}

		c.SetParamValues(unescapedPathParams...)

		return next(c)
	}
}
