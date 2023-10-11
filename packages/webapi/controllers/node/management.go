package node

import (
	"net/http"

	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/packages/webapi/models"

	"github.com/labstack/echo/v4"
)

func (c *Controller) nodeOwnerCertificate(e echo.Context) error {
	certificateBytes := c.nodeService.NodeOwnerCertificate()

	response := models.NodeOwnerCertificateResponse{
		Certificate: hexutil.EncodeHex(certificateBytes),
	}

	return e.JSON(http.StatusOK, response)
}

func (c *Controller) shutdownNode(e echo.Context) error {
	c.nodeService.ShutdownNode()
	return e.NoContent(http.StatusOK)
}
