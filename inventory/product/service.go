package product

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AMETORY/ametory-erp-modules/context"
	"github.com/AMETORY/ametory-erp-modules/file"
	"github.com/AMETORY/ametory-erp-modules/shared/models"
	"github.com/AMETORY/ametory-erp-modules/utils"
	"github.com/morkid/paginate"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProductService struct {
	db          *gorm.DB
	ctx         *context.ERPContext
	fileService *file.FileService
	tagService  *TagService
}

func NewProductService(db *gorm.DB, ctx *context.ERPContext, fileService *file.FileService, tagService *TagService) *ProductService {
	return &ProductService{db: db, ctx: ctx, fileService: fileService, tagService: tagService}
}

func Migrate(db *gorm.DB) error {
	// db.Migrator().AlterColumn(&models.VariantModel{}, "product_id")
	return db.AutoMigrate(
		&models.ProductModel{},
		&models.ProductCategoryModel{},
		&models.MasterProductModel{},
		&models.PriceCategoryModel{},
		&models.PriceModel{},
		&models.MasterProductPriceModel{},
		&models.VariantModel{},
		&models.VariantProductAttributeModel{},
		&models.ProductAttributeModel{},
		&models.ProductMerchant{},
		&models.DiscountModel{},
		&models.TagModel{},
		&models.ProductTag{},
		&models.VariantTag{},
		&models.VarianMerchant{},
		&models.ProductFeedbackModel{},
	)
}

func (s *ProductService) CreateProduct(data *models.ProductModel) error {
	return s.db.Create(data).Error
}

func (s *ProductService) CreateOrUpdateProduct(data *models.ProductModel) error {
	var product models.ProductModel
	err := s.db.Where("id = ?", data.ID).First(&product).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.db.Create(data).Error
		}
		return err
	}
	data.ID = product.ID
	return s.db.Save(data).Error
}

func (s *ProductService) UpdateProduct(data *models.ProductModel) error {
	return s.db.Omit(clause.Associations).Save(data).Error
}

func (s *ProductService) DeleteProduct(id string) error {
	return s.db.Where("id = ?", id).Delete(&models.ProductModel{}).Error
}

func (s *ProductService) GetVariantByID(variantID string, request *http.Request) (*models.VariantModel, error) {
	var variant models.VariantModel
	err := s.db.Where("id = ?", variantID).First(&variant).Error
	if err != nil {
		return nil, err
	}
	merchantID := request.Header.Get("ID-Merchant")
	if merchantID == "" {
		return nil, errors.New("merchant not found")
	}
	// price := s.GetVariantPrice(merchantID, &variant)
	// variant.Price = price
	variant.GetPriceAndDiscount(s.db)

	return &variant, nil
}

func (s *ProductService) GetVariantPrice(merchantID string, variant *models.VariantModel) float64 {
	var variantMerchant models.VarianMerchant
	price := variant.Price
	err := s.db.Where("variant_id = ? AND merchant_id = ?", variant.ID, merchantID).First(&variantMerchant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return price
	}

	price = variantMerchant.Price
	variant.LastUpdatedStock = variantMerchant.LastUpdatedStock
	variant.LastStock = variantMerchant.LastStock
	discountedPrice, _, _, _, err := s.CalculateDiscountedPrice(variant.ProductID, price)
	if err == nil {
		price = discountedPrice
	}
	return price
}
func (s *ProductService) GetProductByID(id string, request *http.Request) (*models.ProductModel, error) {
	idMerchant := ""
	if request != nil {
		if request.Header != nil {
			idMerchant = request.Header.Get("ID-Merchant")
		}
	} else {
		return nil, errors.New("request is nil")
	}
	var product models.ProductModel
	err := s.db.Preload("Tags").Preload("Company").Preload("Variants").Preload("Tax").Preload("MasterProduct").Preload("Category", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name")
	}).Preload("Brand", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name")
	}).Where("id = ?", id).First(&product).Error
	product.Prices, _ = s.ListPricesOfProduct(product.ID)
	product.ProductImages, _ = s.ListImagesOfProduct(product.ID)
	var warehouseID *string
	if request != nil {
		warehouseIDStr := request.Header.Get("ID-Warehouse")
		if warehouseIDStr != "" {
			warehouseID = &warehouseIDStr
		}
	}
	stock, _ := s.GetStock(product.ID, request, warehouseID)

	product.TotalStock = stock
	for i, v := range product.Variants {
		if idMerchant != "" {
			v.MerchantID = &idMerchant
		}
		v.GetPriceAndDiscount(s.db)
		variantStock, _ := s.GetVariantStock(product.ID, v.ID, request, warehouseID)
		v.TotalStock = variantStock
		product.Variants[i] = v
		fmt.Println("VARIANT STOCK", v.ID, variantStock)
	}
	if idMerchant != "" {
		product.MerchantID = &idMerchant
	}
	product.GetPriceAndDiscount(s.db)
	return &product, err
}

func (s *ProductService) GetProductByCode(code string) (*models.ProductModel, error) {
	var product models.ProductModel
	err := s.db.Preload("Category", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name")
	}).Preload("Brand", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name")
	}).Where("sku = ?", code).First(&product).Error
	product.Prices, _ = s.ListPricesOfProduct(product.ID)
	product.ProductImages, _ = s.ListImagesOfProduct(product.ID)
	product.GetPriceAndDiscount(s.db)
	return &product, err
}

func (s *ProductService) GetProductBySku(sku string) (*models.ProductModel, error) {
	var product models.ProductModel
	err := s.db.Preload("Category", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name")
	}).Preload("Brand", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name")
	}).Where("sku = ?", sku).First(&product).Error
	if err != nil {
		return nil, err
	}
	product.Prices, _ = s.ListPricesOfProduct(product.ID)
	product.ProductImages, _ = s.ListImagesOfProduct(product.ID)
	product.GetPriceAndDiscount(s.db)
	return &product, nil
}

func (s *ProductService) GetProducts(request http.Request, search string, status *string) (paginate.Page, error) {
	pg := paginate.New()
	stmt := s.db.Preload("Tags").Preload("Variants.Attributes.Attribute").Preload("Company").Preload("Category", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name")
	}).Preload("Brand", func(db *gorm.DB) *gorm.DB {
		return db.Select("id", "name")
	}).Preload("Tax")
	stmt = stmt.Joins("LEFT JOIN brands ON brands.id = products.brand_id")
	stmt = stmt.Joins("LEFT JOIN product_categories ON product_categories.id = products.category_id")
	stmt = stmt.Joins("LEFT JOIN product_variants ON product_variants.product_id = products.id")
	stmt = stmt.Joins("LEFT JOIN product_tags ON product_tags.product_model_id = products.id")
	stmt = stmt.Joins("LEFT JOIN tags ON product_tags.tag_model_id = tags.id")
	if search != "" {
		stmt = stmt.Where("products.description ILIKE ? OR products.sku ILIKE ? OR products.name ILIKE ? OR products.barcode ILIKE ? OR brands.name ILIKE ? OR product_categories.name ILIKE ? OR product_variants.display_name ILIKE ?",
			"%"+search+"%",
			"%"+search+"%",
			"%"+search+"%",
			"%"+search+"%",
			"%"+search+"%",
			"%"+search+"%",
			"%"+search+"%",
		)
	}
	if request.Header.Get("ID-Company") != "" {
		if request.Header.Get("ID-Company") == "nil" || request.Header.Get("ID-Company") == "null" {
			stmt = stmt.Where("products.company_id is null")
		} else {
			stmt = stmt.Where("products.company_id = ?", request.Header.Get("ID-Company"))

		}
	}
	if request.Header.Get("ID-Distributor") != "" {
		stmt = stmt.Where("products.company_id = ?", request.Header.Get("ID-Distributor"))
	}
	if request.Header.Get("status") != "" {
		stmt = stmt.Where("products.status = ?", request.Header.Get("status"))
	}

	if request.URL.Query().Get("brand_id") != "" {
		stmt = stmt.Where("products.brand_id = ?", request.URL.Query().Get("brand_id"))
	}
	if request.URL.Query().Get("category_id") != "" {
		stmt = stmt.Where("products.category_id = ?", request.URL.Query().Get("category_id"))
	}
	if request.URL.Query().Get("order") != "" {
		stmt = stmt.Order(request.URL.Query().Get("order"))
	}
	if request.URL.Query().Get("status") != "" {
		stmt = stmt.Where("products.status = ?", request.URL.Query().Get("status"))
	}
	if request.URL.Query().Get("category_ids") != "" {
		stmt = stmt.Where("products.category_id IN (?)", strings.Split(request.URL.Query().Get("category_ids"), ","))
	}

	if status != nil {
		stmt = stmt.Where("products.status = ?", status)
	}

	stmt = stmt.Distinct("products.id")
	stmt = stmt.Select("products.*")
	stmt = stmt.Model(&models.ProductModel{})
	utils.FixRequest(&request)
	page := pg.With(stmt).Request(request).Response(&[]models.ProductModel{})
	page.Page = page.Page + 1
	items := page.Items.(*[]models.ProductModel)
	newItems := make([]models.ProductModel, 0)
	var warehouseID *string
	warehouseIDStr := request.Header.Get("ID-Warehouse")
	if warehouseIDStr != "" {
		warehouseID = &warehouseIDStr
	}

	for _, item := range *items {
		item.GetPriceAndDiscount(s.db)
		activeDiscount, _ := s.GetFirstActiveDiscount(item.ID)
		if activeDiscount.ID != "" {
			item.ActiveDiscount = activeDiscount
		}
		img, err := s.ListImagesOfProduct(item.ID)
		if err == nil {
			item.ProductImages = img
		}
		prices, err := s.ListPricesOfProduct(item.ID)
		if err == nil {
			item.Prices = prices
		}

		salesCount, _ := s.GetSalesCount(item.ID, &request, warehouseID)
		item.SalesCount = salesCount
		stock, _ := s.GetStock(item.ID, &request, warehouseID)
		item.TotalStock = stock

		item.TotalStock = stock
		for i, variant := range item.Variants {
			variant.GetPriceAndDiscount(s.db)
			variantStock, _ := s.GetVariantStock(item.ID, variant.ID, &request, warehouseID)
			variant.TotalStock = variantStock
			salesCount, _ := s.GetSalesVariantCount(item.ID, variant.ID, &request, warehouseID)
			variant.SalesCount = salesCount
			// variant.Price = s.GetVariantPrice(merchantID, &variant)
			item.Variants[i] = variant
			fmt.Println("VARIANT STOCK", variant.ID, variant.TotalStock)
		}
		newItems = append(newItems, item)
	}
	page.Items = &newItems
	return page, nil
}

func (s *ProductService) GetProductFeedbacks(productID string, variantID *string, request *http.Request, status []string) (paginate.Page, error) {
	pg := paginate.New()
	stmt := s.db
	if variantID == nil {
		stmt = stmt.Where("product_id = ?", productID)
	} else {
		stmt = stmt.Where("product_id = ? AND variant_id = ?", productID, *variantID)
	}
	if len(status) > 0 {
		stmt = stmt.Where("status IN (?)", status)
	}
	stmt = stmt.Model(&models.ProductFeedbackModel{})
	utils.FixRequest(request)
	page := pg.With(stmt).Request(request).Response(&[]models.ProductFeedbackModel{})
	page.Page = page.Page + 1
	return page, nil
}

func (s *ProductService) CountProductByMerchantID(merchantID string) (int64, error) {
	var count int64
	err := s.db.Model(&models.ProductMerchant{}).Where("merchant_model_id = ?", merchantID).Count(&count).Error
	return count, err
}

func (s *ProductService) CreatePriceCategory(data *models.PriceCategoryModel) error {
	return s.db.Create(data).Error
}

func (s *ProductService) AddPriceToProduct(productID string, data *models.PriceModel) error {
	if data.PriceCategoryID == "" {
		return errors.New("price category id is required")
	}
	data.ProductID = productID
	return s.db.Create(data).Error
}

func (s *ProductService) ListPricesOfProduct(productID string) ([]models.PriceModel, error) {
	var prices []models.PriceModel
	err := s.db.Preload("PriceCategory").Where("product_id = ?", productID).Find(&prices).Error
	return prices, err
}

func (s *ProductService) ListImagesOfProduct(productID string) ([]models.FileModel, error) {
	var images []models.FileModel
	err := s.db.Where("ref_id = ? and ref_type = ?", productID, "product").Find(&images).Error
	return images, err
}

func (s *ProductService) DeletePriceOfProduct(productID string, priceID string) error {
	return s.db.Where("product_id = ? and id = ?", productID, priceID).Delete(&models.PriceModel{}).Error
}

func (s *ProductService) DeleteImageOfProduct(productID string, imageID string) error {
	return s.db.Where("ref_id = ? and ref_type = ? and id = ?", productID, "product", imageID).Delete(&models.FileModel{}).Error
}

func (s *ProductService) GetStock(productID string, request *http.Request, warehouseID *string) (float64, error) {

	var totalStock float64
	db := s.db.Model(&models.StockMovementModel{})
	if request != nil {
		if request.Header.Get("ID-Company") != "" {
			db = db.Where("company_id = ?", request.Header.Get("ID-Company"))
		}
		if request.Header.Get("ID-Distributor") != "" {
			db = db.Where("company_id = ?", request.Header.Get("ID-Distributor"))
		}
	}

	if warehouseID != nil {
		db = db.Where("warehouse_id = ?", *warehouseID)
	}

	if err := db.
		Where("product_id = ?", productID).
		Select("COALESCE(SUM(quantity * value), 0)").
		Scan(&totalStock).Error; err != nil {
		return 0, err
	}

	return totalStock, nil
}
func (s *ProductService) GetSalesCount(productID string, request *http.Request, warehouseID *string) (float64, error) {

	var totalStock float64
	db := s.db.Model(&models.StockMovementModel{})
	if request != nil {
		if request.Header.Get("ID-Company") != "" {
			if request.Header.Get("ID-Company") == "nil" || request.Header.Get("ID-Company") == "null" {
				db = db.Where("company_id is null")
			} else {
				db = db.Where("company_id = ?", request.Header.Get("ID-Company"))

			}
		}
		if request.Header.Get("ID-Distributor") != "" {
			db = db.Where("company_id = ?", request.Header.Get("ID-Distributor"))
		}
	}

	if warehouseID != nil {
		db = db.Where("warehouse_id = ?", *warehouseID)
	}
	db = db.Where("type in (?)", []models.MovementType{models.MovementTypeReturn, models.MovementTypeSale})

	if err := db.
		Where("product_id = ?", productID).
		Select("COALESCE(SUM(quantity * value), 0)").
		Scan(&totalStock).Error; err != nil {
		return 0, err
	}

	return -totalStock, nil
}
func (s *ProductService) GetSalesVariantCount(productID, variantID string, request *http.Request, warehouseID *string) (float64, error) {

	var totalStock float64
	db := s.db.Model(&models.StockMovementModel{})
	if request != nil {
		if request.Header.Get("ID-Company") != "" {
			if request.Header.Get("ID-Company") == "nil" || request.Header.Get("ID-Company") == "null" {
				db = db.Where("company_id is null")
			} else {
				db = db.Where("company_id = ?", request.Header.Get("ID-Company"))

			}
		}
		if request.Header.Get("ID-Distributor") != "" {
			db = db.Where("company_id = ?", request.Header.Get("ID-Distributor"))
		}
	}

	if warehouseID != nil {
		db = db.Where("warehouse_id = ?", *warehouseID)
	}
	db = db.Where("type in (?)", []models.MovementType{models.MovementTypeReturn, models.MovementTypeSale})

	if err := db.
		Where("product_id = ? and variant_id = ?", productID, variantID).
		Select("COALESCE(SUM(quantity * value), 0)").
		Scan(&totalStock).Error; err != nil {
		return 0, err
	}

	return -totalStock, nil
}

func (s *ProductService) GetVariantStock(productID string, variantID string, request *http.Request, warehouseID *string) (float64, error) {

	var totalStock float64
	db := s.db.Model(&models.StockMovementModel{})
	if request != nil {
		if request.Header.Get("ID-Company") != "" {
			if request.Header.Get("ID-Company") == "nil" || request.Header.Get("ID-Company") == "null" {
				db = db.Where("company_id is null")
			} else {
				db = db.Where("company_id = ?", request.Header.Get("ID-Company"))

			}
		}
		if request.Header.Get("ID-Distributor") != "" {
			db = db.Where("company_id = ?", request.Header.Get("ID-Distributor"))
		}
	}

	if warehouseID != nil {
		db = db.Where("warehouse_id = ?", *warehouseID)
	}

	if err := db.
		Where("product_id = ? AND variant_id = ?", productID, variantID).
		Select("COALESCE(SUM(quantity * value), 0)").
		Scan(&totalStock).Error; err != nil {
		return 0, err
	}

	return totalStock, nil
}

func (s *ProductService) GetProductsByMerchant(merchantID string, productIDs []string) ([]models.ProductModel, error) {
	var products []models.ProductModel
	db := s.db.Where("merchant_id = ?", merchantID)
	if len(productIDs) > 0 {
		db = db.Where("id in (?)", productIDs)
	}
	err := db.Find(&products).Error
	for i, v := range products {
		v.GetPriceAndDiscount(s.db)
		products[i] = v
	}
	return products, err
}

func (s *ProductService) CreateProductVariant(data *models.VariantModel) error {
	return s.db.Create(data).Error
}
func (s *ProductService) AddProductUnit(data *models.ProductUnitData) error {
	return s.db.Create(data).Error
}

func (s *ProductService) DeleteProductUnit(productID, unitID string) error {
	return s.db.Where("product_model_id = ? and unit_model_id = ?", productID, unitID).Unscoped().Delete(&models.ProductUnitData{}).Error
}
func (s *ProductService) GetProductVariants(productID string, request http.Request) ([]models.VariantModel, error) {
	var variants []models.VariantModel
	err := s.db.Preload("Attributes.Attribute").Preload("Tags").Where("product_id = ?", productID).Find(&variants).Error
	var warehouseID *string
	warehouseIDStr := request.Header.Get("ID-Warehouse")
	if warehouseIDStr != "" {
		warehouseID = &warehouseIDStr
	}
	for i, v := range variants {
		v.GetPriceAndDiscount(s.db)
		variantStock, _ := s.GetVariantStock(productID, v.ID, &request, warehouseID)
		v.TotalStock = variantStock
		variants[i] = v
		fmt.Println("VARIANT STOCK", v.ID, variantStock)
	}
	return variants, err
}

func (s *ProductService) UpdateProductVariant(data *models.VariantModel) error {
	return s.db.Save(data).Error
}

func (s *ProductService) DeleteProductVariant(id string) error {
	return s.db.Where("id = ?", id).Unscoped().Delete(&models.VariantModel{}).Error
}

func (s *ProductService) AddDiscount(productID string, discountType models.DiscountType, value float64, startDate time.Time, endDate *time.Time) (*models.DiscountModel, error) {
	// Validasi tanggal jika endDate tidak nil
	if endDate != nil && startDate.After(*endDate) {
		return nil, errors.New("start date must be before end date")
	}

	// Buat diskon
	discount := models.DiscountModel{
		ProductID: productID,
		Type:      discountType,
		Value:     value,
		StartDate: startDate,
		EndDate:   endDate,
		IsActive:  true,
	}
	if err := s.db.Create(&discount).Error; err != nil {
		return nil, err
	}

	return &discount, nil
}

func (s *ProductService) DeleteDiscount(productID, discountID string) error {
	return s.db.Where("id = ? and product_id =?", discountID, productID).Unscoped().Delete(&models.DiscountModel{}).Error
}
func (s *ProductService) GetFirstActiveDiscount(productID string) (*models.DiscountModel, error) {
	var discount *models.DiscountModel
	err := s.db.Where("product_id = ? AND is_active = ? AND start_date <= ?", productID, true, time.Now()).
		Where("end_date IS NULL OR end_date >= ?", time.Now()).Order("created_at DESC").
		First(&discount).Error
	return discount, err
}

func (s *ProductService) GetActiveDiscounts(productID string) ([]models.DiscountModel, error) {
	var discounts []models.DiscountModel
	err := s.db.Where("product_id = ? AND is_active = ? AND start_date <= ?", productID, true, time.Now()).
		Where("end_date IS NULL OR end_date >= ?", time.Now()).Order("created_at DESC").
		Find(&discounts).Error
	return discounts, err
}

func (s *ProductService) GetAllDiscountByProductID(productID string) ([]models.DiscountModel, error) {
	var discounts []models.DiscountModel
	err := s.db.Where("product_id = ?", productID).Find(&discounts).Error
	return discounts, err
}

func (s *ProductService) GetBestDealByPercentage(limit int) ([]models.ProductModel, error) {
	var products []models.ProductModel
	err := s.db.Joins("JOIN discounts ON products.id = discounts.product_id").
		Where("discounts.type = ? AND discounts.is_active = ? AND discounts.start_date <= ?", models.DiscountPercentage, true, time.Now()).
		Where("discounts.end_date IS NULL OR discounts.end_date >= ?", time.Now()).
		Order("discounts.value DESC").
		Limit(limit).
		Find(&products).Error
	return products, err
}

func (s *ProductService) GetBestDealByAmount(limit int) ([]models.ProductModel, error) {
	var products []models.ProductModel
	err := s.db.Joins("JOIN discounts ON products.id = discounts.product_id").
		Where("discounts.type = ? AND discounts.is_active = ? AND discounts.start_date <= ?", models.DiscountAmount, true, time.Now()).
		Where("discounts.end_date IS NULL OR discounts.end_date >= ?", time.Now()).
		Order("discounts.value DESC").
		Limit(limit).
		Find(&products).Error
	return products, err
}

func (s *ProductService) GetBestDealByDiscountedPrice(limit int) ([]models.ProductModel, error) {
	var products []models.ProductModel
	err := s.db.Joins("JOIN discounts ON products.id = discounts.product_id").
		Where("discounts.is_active = ? AND discounts.start_date <= ?", true, time.Now()).
		Where("discounts.end_date IS NULL OR discounts.end_date >= ?", time.Now()).
		Select("products.*, (products.price - CASE WHEN discounts.type = 'PERCENTAGE' THEN products.price * discounts.value / 100 ELSE discounts.value END) as discounted_price").
		Order("discounted_price ASC").
		Limit(limit).
		Find(&products).Error
	return products, err
}

func (s *ProductService) CalculateDiscountedPrice(productID string, originalPrice float64) (float64, float64, float64, string, error) {
	fmt.Println("ORIGINAL_PRICE", originalPrice)
	// Dapatkan diskon aktif untuk produk
	discounts, err := s.GetActiveDiscounts(productID)
	if err != nil {
		return 0, 0, 0, "", err
	}

	if len(discounts) == 0 {
		return originalPrice, 0, 0, "", nil
	}

	// Hitung harga setelah diskon
	discountedPrice := originalPrice
	discount := discounts[0]
	discAmount := float64(0)
	switch discount.Type {
	case models.DiscountPercentage:
		discAmount = originalPrice * (discount.Value / 100)
		discountedPrice -= discAmount
	case models.DiscountAmount:
		discAmount = discount.Value
		discountedPrice -= discAmount
	}

	// Pastikan harga tidak negatif
	if discountedPrice < 0 {
		discountedPrice = 0
	}

	fmt.Println("DISCOUNT_AMOUNT", discAmount)

	return discountedPrice, discAmount, discount.Value, string(discount.Type), nil
}

func (s *ProductService) UpdateDiscount(discountID string, data models.DiscountModel) error {
	// Validasi tanggal
	if data.EndDate != nil && data.StartDate.After(*data.EndDate) {
		return errors.New("start date must be before end date")
	}

	return s.db.Model(&models.DiscountModel{}).Where("id = ?", discountID).Save(&data).Error
}

// DeactivateDiscount: Menonaktifkan diskon
func (s *ProductService) DeactivateDiscount(discountID string) error {
	return s.db.Model(&models.DiscountModel{}).Where("id = ?", discountID).Update("is_active", false).Error
}

func (s *ProductService) AddProductTagByName(productID string, name string) (*models.TagModel, error) {
	tag, err := s.tagService.GetTagByName(name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			tag = &models.TagModel{
				Name:  name,
				Color: "#F5F5F5",
			}
			err = s.db.Create(tag).Error
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	var product models.ProductModel
	s.db.Find(&product, "id = ?", productID)

	// Tambahkan tag ke produk
	err = s.db.Model(&product).Association("Tags").Append(tag)
	return tag, err
}
func (s *ProductService) AddVariantTagByName(productID string, name string) error {
	tag, err := s.tagService.GetTagByName(name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			tag = &models.TagModel{
				Name: name,
			}
			err = s.db.Create(tag).Error
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Tambahkan tag ke produk
	err = s.db.Model(&models.VariantModel{}).Where("id = ?", productID).Association("Tags").Append(tag)
	return err
}

func (s *ProductService) CreateProductFeedback(data *models.ProductFeedbackModel) error {
	return s.db.Create(data).Error
}

func (s *ProductService) DeleteProductFeedback(id string) error {
	return s.db.Delete(&models.ProductFeedbackModel{}, "id = ?", id).Error
}

func (s *ProductService) GetBestSellingProduct(request *http.Request, limit int, warehouseID *string) ([]models.PopularProduct, error) {
	var results []models.PopularProduct
	db := s.db.Model(&models.StockMovementModel{})
	if request != nil {
		if request.Header.Get("ID-Company") != "" {
			if request.Header.Get("ID-Company") == "nil" || request.Header.Get("ID-Company") == "null" {
				db = db.Where("products.company_id is null")
			} else {
				db = db.Where("products.company_id = ?", request.Header.Get("ID-Company"))

			}
		}
		// if request.Header.Get("ID-Distributor") != "" {
		// 	db = db.Where("products.company_id = ?", request.Header.Get("ID-Distributor"))
		// }
	}

	if warehouseID != nil {
		db = db.Where("warehouse_id = ?", *warehouseID)
	}
	db = db.Where("type in (?)", []models.MovementType{models.MovementTypeReturn, models.MovementTypeSale})

	if err := db.
		Joins("JOIN products on stock_movements.product_id = products.id").
		Select("products.id", "products.display_name", "COALESCE(SUM(quantity * value), 0) * -1 as total_sale").
		Group("products.id").
		Order("COALESCE(SUM(quantity * value), 0) DESC").
		Limit(limit).
		Scan(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
