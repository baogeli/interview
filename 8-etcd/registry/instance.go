package registry

import (
	"encoding/json"
	"fmt"
	"path"
)

// Namespace 是所有服务注册信息在 etcd 中的根前缀。
// 完整 key 规范:  /services/{服务名}/{实例ID}
// value 为 ServiceInstance 的 JSON 序列化结果。
//
// 之所以用「前缀树」形式组织 key，是因为 etcd v3 的 key 是扁平的有序字节串，
// WithPrefix() 本质是一次 [prefix, prefixEnd) 的范围查询/监听，
// 所以只要按前缀设计 key，就能一次性拿到/监听某个服务下的全部实例。
const Namespace = "/services"

// ServiceInstance 描述一个服务实例的注册元数据。
type ServiceInstance struct {
	Name     string            `json:"name"`               // 服务名，如 user-service
	ID       string            `json:"id"`                 // 实例唯一 ID，如 user-service-1
	Addr     string            `json:"addr"`               // 实例地址 host:port
	Weight   int               `json:"weight,omitempty"`   // 权重，供上层负载均衡使用
	Metadata map[string]string `json:"metadata,omitempty"` // 扩展元数据：版本、机房、灰度标签等
}

// Key 返回该实例在 etcd 中的完整 key。
func (s ServiceInstance) Key() string {
	return path.Join(Namespace, s.Name, s.ID)
}

// String 便于日志打印。
func (s ServiceInstance) String() string {
	return fmt.Sprintf("%s(%s)", s.ID, s.Addr)
}

// Marshal 把实例元数据序列化为 etcd 的 value。
func (s ServiceInstance) Marshal() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("marshal instance: %w", err)
	}
	return string(b), nil
}

// UnmarshalInstance 从 etcd 的 value 反序列化实例元数据。
func UnmarshalInstance(v []byte) (ServiceInstance, error) {
	var inst ServiceInstance
	if err := json.Unmarshal(v, &inst); err != nil {
		return inst, fmt.Errorf("unmarshal instance: %w", err)
	}
	return inst, nil
}

// ServicePrefix 返回某个服务名对应的 etcd key 前缀。
//
// 注意结尾必须补 "/"：否则监听 /services/user 会同时命中 /services/user-admin，
// 这是服务发现里非常典型的一个「前缀污染」bug。
func ServicePrefix(name string) string {
	return path.Join(Namespace, name) + "/"
}
