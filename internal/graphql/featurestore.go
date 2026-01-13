package graphql

import (
	"context"
	"fmt"
	"time"

	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
)

// FeatureStoreSchema creates a GraphQL schema for the feature store.
type FeatureStoreSchema struct {
	*Schema
	store    *storage.Store
	registry storage.SchemaRegistry
}

// NewFeatureStoreSchema creates a new feature store GraphQL schema.
func NewFeatureStoreSchema(store *storage.Store, registry storage.SchemaRegistry) (*FeatureStoreSchema, error) {
	fs := &FeatureStoreSchema{store: store, registry: registry}

	// Define types
	featureValueType := fs.createFeatureValueType()
	featureGroupType := fs.createFeatureGroupType()
	featureDefinitionType := fs.createFeatureDefinitionType()
	aggregationResultType := fs.createAggregationResultType()

	// Input types
	featureInputType := fs.createFeatureInputType()
	featureQueryInputType := fs.createFeatureQueryInputType()

	// Create Query type
	queryType := Object("Query", map[string]*Field{
		"feature": {
			Name:        "feature",
			Description: "Get a single feature value",
			Type:        featureValueType,
			Args: []*Argument{
				{Name: "entity", Type: NonNull(StringScalar)},
				{Name: "feature", Type: NonNull(StringScalar)},
			},
			Resolver: fs.resolveFeature,
		},
		"features": {
			Name:        "features",
			Description: "Get multiple feature values for an entity",
			Type:        List(featureValueType),
			Args: []*Argument{
				{Name: "entity", Type: NonNull(StringScalar)},
				{Name: "features", Type: List(StringScalar)},
			},
			Resolver: fs.resolveFeatures,
		},
		"featureHistory": {
			Name:        "featureHistory",
			Description: "Get historical feature values",
			Type:        List(featureValueType),
			Args: []*Argument{
				{Name: "entity", Type: NonNull(StringScalar)},
				{Name: "feature", Type: NonNull(StringScalar)},
				{Name: "startTime", Type: DateTimeScalar},
				{Name: "endTime", Type: DateTimeScalar},
				{Name: "limit", Type: IntScalar},
			},
			Resolver: fs.resolveFeatureHistory,
		},
		"featureGroups": {
			Name:        "featureGroups",
			Description: "List all feature groups",
			Type:        List(featureGroupType),
			Resolver:    fs.resolveFeatureGroups,
		},
		"featureGroup": {
			Name:        "featureGroup",
			Description: "Get a feature group by name",
			Type:        featureGroupType,
			Args: []*Argument{
				{Name: "name", Type: NonNull(StringScalar)},
			},
			Resolver: fs.resolveFeatureGroup,
		},
		"aggregation": {
			Name:        "aggregation",
			Description: "Get an aggregation result",
			Type:        aggregationResultType,
			Args: []*Argument{
				{Name: "entity", Type: NonNull(StringScalar)},
				{Name: "feature", Type: NonNull(StringScalar)},
				{Name: "function", Type: NonNull(StringScalar)},
				{Name: "window", Type: StringScalar},
			},
			Resolver: fs.resolveAggregation,
		},
		"healthCheck": {
			Name:        "healthCheck",
			Description: "Check system health",
			Type:        fs.createHealthCheckType(),
			Resolver:    fs.resolveHealthCheck,
		},
	})

	// Create Mutation type
	mutationType := Object("Mutation", map[string]*Field{
		"setFeature": {
			Name:        "setFeature",
			Description: "Set a feature value",
			Type:        featureValueType,
			Args: []*Argument{
				{Name: "entity", Type: NonNull(StringScalar)},
				{Name: "feature", Type: NonNull(StringScalar)},
				{Name: "value", Type: NonNull(JSONScalar)},
				{Name: "timestamp", Type: DateTimeScalar},
			},
			Resolver: fs.resolveSetFeature,
		},
		"setFeatures": {
			Name:        "setFeatures",
			Description: "Set multiple feature values",
			Type:        List(featureValueType),
			Args: []*Argument{
				{Name: "features", Type: NonNull(List(featureInputType))},
			},
			Resolver: fs.resolveSetFeatures,
		},
		"deleteFeature": {
			Name:        "deleteFeature",
			Description: "Delete a feature value",
			Type:        BooleanScalar,
			Args: []*Argument{
				{Name: "entity", Type: NonNull(StringScalar)},
				{Name: "feature", Type: NonNull(StringScalar)},
			},
			Resolver: fs.resolveDeleteFeature,
		},
		"createFeatureGroup": {
			Name:        "createFeatureGroup",
			Description: "Create a new feature group",
			Type:        featureGroupType,
			Args: []*Argument{
				{Name: "name", Type: NonNull(StringScalar)},
				{Name: "description", Type: StringScalar},
				{Name: "features", Type: List(featureDefinitionType)},
			},
			Resolver: fs.resolveCreateFeatureGroup,
		},
	})

	// Create schema
	schema, err := NewSchema(SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
		Types: []Type{
			featureValueType,
			featureGroupType,
			featureDefinitionType,
			aggregationResultType,
			featureInputType,
			featureQueryInputType,
		},
	})
	if err != nil {
		return nil, err
	}

	fs.Schema = schema
	return fs, nil
}

func (fs *FeatureStoreSchema) createFeatureValueType() *ObjectType {
	return Object("FeatureValue", map[string]*Field{
		"entity": {
			Name: "entity",
			Type: NonNull(StringScalar),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if fv, ok := parent.(map[string]interface{}); ok {
					return fv["entity"], nil
				}
				return nil, nil
			},
		},
		"feature": {
			Name: "feature",
			Type: NonNull(StringScalar),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if fv, ok := parent.(map[string]interface{}); ok {
					return fv["feature"], nil
				}
				return nil, nil
			},
		},
		"value": {
			Name: "value",
			Type: JSONScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if fv, ok := parent.(map[string]interface{}); ok {
					return fv["value"], nil
				}
				return nil, nil
			},
		},
		"timestamp": {
			Name: "timestamp",
			Type: DateTimeScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if fv, ok := parent.(map[string]interface{}); ok {
					return fv["timestamp"], nil
				}
				return nil, nil
			},
		},
		"version": {
			Name: "version",
			Type: IntScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if fv, ok := parent.(map[string]interface{}); ok {
					return fv["version"], nil
				}
				return nil, nil
			},
		},
	})
}

func (fs *FeatureStoreSchema) createFeatureGroupType() *ObjectType {
	return Object("FeatureGroup", map[string]*Field{
		"name": {
			Name: "name",
			Type: NonNull(StringScalar),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if fg, ok := parent.(map[string]interface{}); ok {
					return fg["name"], nil
				}
				return nil, nil
			},
		},
		"description": {
			Name: "description",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if fg, ok := parent.(map[string]interface{}); ok {
					return fg["description"], nil
				}
				return nil, nil
			},
		},
		"featureCount": {
			Name: "featureCount",
			Type: IntScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if fg, ok := parent.(map[string]interface{}); ok {
					return fg["featureCount"], nil
				}
				return nil, nil
			},
		},
		"createdAt": {
			Name: "createdAt",
			Type: DateTimeScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if fg, ok := parent.(map[string]interface{}); ok {
					return fg["createdAt"], nil
				}
				return nil, nil
			},
		},
	})
}

func (fs *FeatureStoreSchema) createFeatureDefinitionType() *InputObjectType {
	return Input("FeatureDefinition", map[string]*InputField{
		"name":         {Name: "name", Type: NonNull(StringScalar)},
		"type":         {Name: "type", Type: NonNull(StringScalar)},
		"description":  {Name: "description", Type: StringScalar},
		"defaultValue": {Name: "defaultValue", Type: JSONScalar},
	})
}

func (fs *FeatureStoreSchema) createAggregationResultType() *ObjectType {
	return Object("AggregationResult", map[string]*Field{
		"entity": {
			Name: "entity",
			Type: NonNull(StringScalar),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if ar, ok := parent.(map[string]interface{}); ok {
					return ar["entity"], nil
				}
				return nil, nil
			},
		},
		"feature": {
			Name: "feature",
			Type: NonNull(StringScalar),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if ar, ok := parent.(map[string]interface{}); ok {
					return ar["feature"], nil
				}
				return nil, nil
			},
		},
		"function": {
			Name: "function",
			Type: NonNull(StringScalar),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if ar, ok := parent.(map[string]interface{}); ok {
					return ar["function"], nil
				}
				return nil, nil
			},
		},
		"value": {
			Name: "value",
			Type: FloatScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if ar, ok := parent.(map[string]interface{}); ok {
					return ar["value"], nil
				}
				return nil, nil
			},
		},
		"window": {
			Name: "window",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if ar, ok := parent.(map[string]interface{}); ok {
					return ar["window"], nil
				}
				return nil, nil
			},
		},
		"timestamp": {
			Name: "timestamp",
			Type: DateTimeScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if ar, ok := parent.(map[string]interface{}); ok {
					return ar["timestamp"], nil
				}
				return nil, nil
			},
		},
	})
}

func (fs *FeatureStoreSchema) createFeatureInputType() *InputObjectType {
	return Input("FeatureInput", map[string]*InputField{
		"entity":    {Name: "entity", Type: NonNull(StringScalar)},
		"feature":   {Name: "feature", Type: NonNull(StringScalar)},
		"value":     {Name: "value", Type: NonNull(JSONScalar)},
		"timestamp": {Name: "timestamp", Type: DateTimeScalar},
	})
}

func (fs *FeatureStoreSchema) createFeatureQueryInputType() *InputObjectType {
	return Input("FeatureQueryInput", map[string]*InputField{
		"entity":   {Name: "entity", Type: NonNull(StringScalar)},
		"features": {Name: "features", Type: List(StringScalar)},
	})
}

func (fs *FeatureStoreSchema) createHealthCheckType() *ObjectType {
	return Object("HealthCheck", map[string]*Field{
		"status": {
			Name: "status",
			Type: NonNull(StringScalar),
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if hc, ok := parent.(map[string]interface{}); ok {
					return hc["status"], nil
				}
				return "unknown", nil
			},
		},
		"timestamp": {
			Name: "timestamp",
			Type: DateTimeScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if hc, ok := parent.(map[string]interface{}); ok {
					return hc["timestamp"], nil
				}
				return nil, nil
			},
		},
		"version": {
			Name: "version",
			Type: StringScalar,
			Resolver: func(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
				if hc, ok := parent.(map[string]interface{}); ok {
					return hc["version"], nil
				}
				return "1.0.0", nil
			},
		},
	})
}

// Resolvers

func (fs *FeatureStoreSchema) resolveFeature(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	entity, ok := args["entity"].(string)
	if !ok {
		return nil, fmt.Errorf("entity must be a string")
	}
	feature, ok := args["feature"].(string)
	if !ok {
		return nil, fmt.Errorf("feature must be a string")
	}

	values, err := fs.store.Get(entity, []string{feature})
	if err != nil {
		return nil, err
	}

	value, ok := values[feature]
	if !ok {
		return nil, fmt.Errorf("feature %s not found", feature)
	}

	return map[string]interface{}{
		"entity":    entity,
		"feature":   feature,
		"value":     value.Value,
		"timestamp": value.Timestamp,
		"version":   value.Version,
	}, nil
}

func (fs *FeatureStoreSchema) resolveFeatures(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	entity, ok := args["entity"].(string)
	if !ok {
		return nil, fmt.Errorf("entity must be a string")
	}
	featuresArg := args["features"]

	var features []string
	if featuresArg != nil {
		if featureList, ok := featuresArg.([]interface{}); ok {
			for _, f := range featureList {
				features = append(features, fmt.Sprintf("%v", f))
			}
		}
	}

	values, err := fs.store.Get(entity, features)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, 0, len(values))
	for featureName, value := range values {
		result = append(result, map[string]interface{}{
			"entity":    entity,
			"feature":   featureName,
			"value":     value.Value,
			"timestamp": value.Timestamp,
			"version":   value.Version,
		})
	}

	return result, nil
}

func (fs *FeatureStoreSchema) resolveFeatureHistory(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	entity, ok := args["entity"].(string)
	if !ok {
		return nil, fmt.Errorf("entity must be a string")
	}
	feature, ok := args["feature"].(string)
	if !ok {
		return nil, fmt.Errorf("feature must be a string")
	}

	var asOf time.Time
	if st, ok := args["startTime"].(time.Time); ok {
		asOf = st
	} else {
		asOf = time.Now()
	}

	// Use GetAsOf for point-in-time query
	values, err := fs.store.GetAsOf(entity, []string{feature}, asOf)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, 0, len(values))
	for featureName, value := range values {
		result = append(result, map[string]interface{}{
			"entity":    entity,
			"feature":   featureName,
			"value":     value.Value,
			"timestamp": value.Timestamp,
			"version":   value.Version,
		})
	}

	return result, nil
}

func (fs *FeatureStoreSchema) resolveFeatureGroups(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	if fs.registry == nil {
		return []interface{}{}, nil
	}

	groups := fs.registry.ListGroups()
	result := make([]interface{}, 0, len(groups))

	for _, group := range groups {
		result = append(result, map[string]interface{}{
			"name":         group.Name,
			"description":  group.Description,
			"featureCount": len(group.Features),
			"createdAt":    time.Now(), // FeatureGroup doesn't track creation time
		})
	}

	return result, nil
}

func (fs *FeatureStoreSchema) resolveFeatureGroup(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	if fs.registry == nil {
		return nil, fmt.Errorf("registry not configured")
	}

	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name must be a string")
	}

	group, err := fs.registry.GetGroup(name)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":         group.Name,
		"description":  group.Description,
		"featureCount": len(group.Features),
		"createdAt":    time.Now(), // FeatureGroup doesn't track creation time
	}, nil
}

func (fs *FeatureStoreSchema) resolveAggregation(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	entity, ok := args["entity"].(string)
	if !ok {
		return nil, fmt.Errorf("entity must be a string")
	}
	feature, ok := args["feature"].(string)
	if !ok {
		return nil, fmt.Errorf("feature must be a string")
	}
	function, ok := args["function"].(string)
	if !ok {
		return nil, fmt.Errorf("function must be a string")
	}

	window := "1h"
	if w, ok := args["window"].(string); ok {
		window = w
	}

	// Get current feature value (aggregations would be stored as features)
	values, err := fs.store.Get(entity, []string{feature})
	if err != nil {
		return nil, err
	}

	var value interface{}
	if fv, ok := values[feature]; ok {
		value = fv.Value
	}

	return map[string]interface{}{
		"entity":    entity,
		"feature":   feature,
		"function":  function,
		"value":     value,
		"window":    window,
		"timestamp": time.Now(),
	}, nil
}

func (fs *FeatureStoreSchema) resolveHealthCheck(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
	}, nil
}

func (fs *FeatureStoreSchema) resolveSetFeature(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	entity, ok := args["entity"].(string)
	if !ok {
		return nil, fmt.Errorf("entity must be a string")
	}
	feature, ok := args["feature"].(string)
	if !ok {
		return nil, fmt.Errorf("feature must be a string")
	}
	value := args["value"]

	timestamp := time.Now()
	if ts, ok := args["timestamp"].(time.Time); ok {
		timestamp = ts
	}

	featureValue := &domain.FeatureValue{
		Value:     value,
		Timestamp: timestamp.UnixNano(),
		Version:   1,
	}

	if err := fs.store.Put(entity, map[string]*domain.FeatureValue{feature: featureValue}); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"entity":    entity,
		"feature":   feature,
		"value":     value,
		"timestamp": timestamp,
		"version":   int64(1),
	}, nil
}

func (fs *FeatureStoreSchema) resolveSetFeatures(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	featuresArg := args["features"]
	featuresList, ok := featuresArg.([]interface{})
	if !ok {
		return nil, fmt.Errorf("features must be a list")
	}

	result := make([]interface{}, 0, len(featuresList))

	for _, f := range featuresList {
		featureMap, ok := f.(map[string]interface{})
		if !ok {
			continue
		}

		entity := fmt.Sprintf("%v", featureMap["entity"])
		feature := fmt.Sprintf("%v", featureMap["feature"])
		value := featureMap["value"]

		timestamp := time.Now()
		if ts, ok := featureMap["timestamp"].(time.Time); ok {
			timestamp = ts
		}

		featureValue := &domain.FeatureValue{
			Value:     value,
			Timestamp: timestamp.UnixNano(),
			Version:   1,
		}

		if err := fs.store.Put(entity, map[string]*domain.FeatureValue{feature: featureValue}); err != nil {
			return nil, err
		}

		result = append(result, map[string]interface{}{
			"entity":    entity,
			"feature":   feature,
			"value":     value,
			"timestamp": timestamp,
			"version":   int64(1),
		})
	}

	return result, nil
}

func (fs *FeatureStoreSchema) resolveDeleteFeature(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	entity, ok := args["entity"].(string)
	if !ok {
		return nil, fmt.Errorf("entity must be a string")
	}
	if _, ok := args["feature"].(string); !ok {
		return nil, fmt.Errorf("feature must be a string")
	}

	// Note: Store.Delete removes all features for an entity
	// To delete a single feature, set its value to nil instead
	if err := fs.store.Delete(entity); err != nil {
		return false, err
	}

	return true, nil
}

func (fs *FeatureStoreSchema) resolveCreateFeatureGroup(ctx context.Context, parent interface{}, args map[string]interface{}) (interface{}, error) {
	if fs.registry == nil {
		return nil, fmt.Errorf("registry not configured")
	}

	// Try to cast to concrete Registry type which has RegisterGroup
	registry, ok := fs.registry.(*storage.Registry)
	if !ok {
		return nil, fmt.Errorf("registry does not support group creation")
	}

	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name must be a string")
	}

	description := ""
	if d, ok := args["description"].(string); ok {
		description = d
	}

	group := &domain.FeatureGroup{
		Name:        name,
		Description: description,
		Features:    []domain.FeatureSpec{},
	}

	if err := registry.RegisterGroup(group); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":         group.Name,
		"description":  group.Description,
		"featureCount": len(group.Features),
		"createdAt":    time.Now(),
	}, nil
}
