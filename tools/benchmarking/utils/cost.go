package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/samber/lo"
)

type PriceItemInstance struct {
	Product ProductInfoInstance `json:"product"`
	Terms   TermsInstance       `json:"terms"`
}

type ProductInfoInstance struct {
	Attributes ProductAttributesInstance `json:"attributes"`
}

type ProductAttributesInstance struct {
	InstanceType string `json:"instanceType"`
}

type TermsInstance struct {
	OnDemand map[string]OnDemandTermInstance `json:"OnDemand"`
}

type OnDemandTermInstance struct {
	PriceDimensions map[string]PriceDimensionInstance `json:"priceDimensions"`
}

type PriceDimensionInstance struct {
	PricePerUnit PricePerUnitInstance `json:"pricePerUnit"`
}

type PricePerUnitInstance struct {
	USD string `json:"USD"`
}

func GetDefaultPricingInput() *pricing.GetProductsInput {
	return &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []types.Filter{
			{
				Field: lo.ToPtr("operatingSystem"),
				Type:  types.FilterTypeTermMatch,
				Value: lo.ToPtr("Linux"),
			},
			{
				Field: lo.ToPtr("marketoption"),
				Type:  types.FilterTypeTermMatch,
				Value: lo.ToPtr("OnDemand"),
			},
			{
				Field: lo.ToPtr("regionCode"),
				Type:  types.FilterTypeTermMatch,
				Value: lo.ToPtr("us-west-2"),
			},
			{
				Field: lo.ToPtr("tenancy"),
				Type:  types.FilterTypeTermMatch,
				Value: lo.ToPtr("shared"),
			},
			{
				Field: aws.String("serviceCode"),
				Type:  types.FilterTypeTermMatch,
				Value: aws.String("AmazonEC2"),
			},
			{
				Field: aws.String("preInstalledSw"),
				Type:  types.FilterTypeTermMatch,
				Value: aws.String("NA"),
			},
			{
				Field: aws.String("capacitystatus"),
				Type:  types.FilterTypeTermMatch,
				Value: aws.String("Used"),
			},
		},
	}
}

func getCostMap(ctx context.Context) map[string]float64 {
	cfg, err := config.LoadDefaultConfig(ctx)
	cfg.Region = "us-east-1"
	if err != nil {
		log.Fatal(err)
	}
	pricingInput := GetDefaultPricingInput()
	costMap := make(map[string]float64)
	client := pricing.NewFromConfig(cfg)
	paginator := pricing.NewGetProductsPaginator(client, pricingInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			panic(err)
		}
		for _, i := range page.PriceList {
			var pricingData PriceItemInstance
			err = json.Unmarshal([]byte(i), &pricingData)
			if err != nil {
				panic(err)
			}

			costMap[pricingData.Product.Attributes.InstanceType], err = getUSDForPriceItem(pricingData)
			if err != nil {
				fmt.Println(err)
			}
		}
	}

	return costMap
}

func getUSDForPriceItem(priceItem PriceItemInstance) (float64, error) {
	for _, odInstance := range priceItem.Terms.OnDemand {
		for _, dimension := range odInstance.PriceDimensions {
			cost, err := strconv.ParseFloat(dimension.PricePerUnit.USD, 64)
			return cost, err
		}
	}
	return 0.0, nil
}
