package policy

import (
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

// ZookeeperDriver is the zookeeper database connector
type ZookeeperDriver struct {
	zkDriver *zk.Conn
}

// Conn connects to zookeeper
func (z *ZookeeperDriver) Conn(hosts string) error {
	hostList := strings.Split(hosts, ",")
	// hostList := []string{"127.0.0.1"}

	var err error
	z.zkDriver, _, err = zk.Connect(hostList, 4*time.Second)
	if err != nil {
		return err
	}
	/*_, __, _, err := zkDriver.ChildrenW("/")
	if err != nil {
		return err
	}
	*/
	return nil
}

// GetPolicy gets the policy
func (z *ZookeeperDriver) GetPolicy(tenantName string) TenantPolicy {
	return TenantPolicy{}
}

// Evaluate gets the policy
func (z *ZookeeperDriver) Evaluate(tenantName string) error {
	return nil
}
