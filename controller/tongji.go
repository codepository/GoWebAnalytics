package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/codepository/GoWebAnalytics/connmgr"
	"github.com/mumushuiding/util"
)

// Test Test
func Test(writer http.ResponseWriter, request *http.Request) {
	var data connmgr.WebData
	err := util.Body2Struct(request, &data)
	data.Browsing.Start = time.Now()
	if err != nil {
		fmt.Fprintln(writer, err)
	}
	connmgr.CM.NewWebData(&data)
}
