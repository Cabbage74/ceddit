package snowflake

import (
	"time"

	"github.com/bwmarrin/snowflake"
)

var node *snowflake.Node

func Init(startTime string, machineID int64) error {
	st, err := time.Parse("2006-01-02", startTime)
	if err != nil {
		return err
	}

	snowflake.Epoch = st.UnixNano() / 1000000
	node, err = snowflake.NewNode(machineID)
	return err
}

func GenID() int64 {
	return node.Generate().Int64()
}
