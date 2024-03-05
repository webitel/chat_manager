package logger

import (
	"context"
	"fmt"
	"time"

	cache "github.com/hashicorp/golang-lru/v2/expirable"
	proto "github.com/webitel/chat_manager/api/proto/logger"
)

const (
	DefaultCacheTimeout = 120 * time.Second
)

type GrpcClient interface {
	Config() ConfigApi
}

type grpcClient struct {
	config ConfigApi
}

func (c *grpcClient) Config() ConfigApi {
	return c.config
}

func NewGrpcClient(conn proto.ConfigService) GrpcClient {
	return &grpcClient{config: NewConfigApi(conn)}
}

type ConfigApi interface {
	CheckIsActive(ctx context.Context, domainId int64, objectName string) (bool, error)
}

func NewConfigApi(client proto.ConfigService) ConfigApi {
	return &configApi{client: client, memoryCache: cache.NewLRU[string, bool](200, nil, DefaultCacheTimeout)}
}

func FormatKey(domainId int64, objectName string) string {
	return fmt.Sprintf("logger.config.%d.%s", domainId, objectName)
}

type configApi struct {
	client      proto.ConfigService
	memoryCache *cache.LRU[string, bool]
}

func (c *configApi) CheckIsActive(ctx context.Context, domainId int64, objectName string) (bool, error) {
	cacheKey := FormatKey(domainId, objectName)
	enabled, ok := c.memoryCache.Get(cacheKey)
	if !ok {
		in := &proto.CheckConfigStatusRequest{
			ObjectName: objectName,
			DomainId:   domainId,
		}
		res, err := c.client.CheckConfigStatus(ctx, in)
		if err != nil {
			return false, err
		}
		c.memoryCache.Add(cacheKey, res.GetIsEnabled())
		return res.GetIsEnabled(), nil
	}
	return enabled, nil
}
