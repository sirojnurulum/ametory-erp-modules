package merchant

import (
	"errors"
	"net/http"

	"github.com/AMETORY/ametory-erp-modules/context"
	"github.com/AMETORY/ametory-erp-modules/finance"
	"github.com/AMETORY/ametory-erp-modules/inventory"
	"github.com/AMETORY/ametory-erp-modules/shared/models"
	"github.com/AMETORY/ametory-erp-modules/utils"
	"github.com/morkid/paginate"
	"gorm.io/gorm"
)

type MerchantService struct {
	ctx            *context.ERPContext
	db             *gorm.DB
	financeService *finance.FinanceService
}

func NewMerchantService(db *gorm.DB, ctx *context.ERPContext, financeService *finance.FinanceService) *MerchantService {
	return &MerchantService{db: db, ctx: ctx, financeService: financeService}
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.MerchantModel{})
}

func (s *MerchantService) GetNearbyMerchants(lat, lng float64, radius float64) ([]models.MerchantModel, error) {
	var merchants []models.MerchantModel

	rows, err := s.db.Raw(`
		SELECT *, (
			6371 * acos(
				cos(radians(?)) * cos(radians(latitude)) * cos(radians(longitude) - radians(?)) +
				sin(radians(?)) * sin(radians(latitude))
			)
		) AS distance
		FROM merchant
		HAVING distance <= ?
		ORDER BY distance
	`, lat, lng, lat, radius).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var merchant models.MerchantModel
		if err := s.db.ScanRows(rows, &merchant); err != nil {
			return nil, err
		}
		merchants = append(merchants, merchant)
	}
	return merchants, err
}

func (s *MerchantService) CreateMerchant(data *models.MerchantModel) error {
	return s.db.Create(data).Error
}

func (s *MerchantService) UpdateMerchant(id string, data *models.MerchantModel) error {
	return s.db.Where("id = ?", id).Updates(data).Error
}

func (s *MerchantService) DeleteMerchant(id string) error {
	return s.db.Where("id = ?", id).Delete(&models.MerchantModel{}).Error
}

func (s *MerchantService) GetMerchantByID(id string) (*models.MerchantModel, error) {
	var invoice models.MerchantModel
	err := s.db.Preload("Company").Preload("User").Where("id = ?", id).First(&invoice).Error
	return &invoice, err
}
func (s *MerchantService) GetActiveMerchantByID(id string) (*models.MerchantModel, error) {
	var invoice models.MerchantModel
	err := s.db.Preload("Company").Preload("User").Preload("DefaultWarehouse").Where("id = ? ", id).First(&invoice).Error
	if invoice.Status == "PENDING" {
		return nil, errors.New("merchant is not active")
	}
	if invoice.Status == "SUSPENDED" {
		return nil, errors.New("merchant is suspended")
	}
	return &invoice, err
}

func (s *MerchantService) GetMerchants(request http.Request, search string) (paginate.Page, error) {
	pg := paginate.New()
	stmt := s.db
	if search != "" {
		stmt = stmt.Where("merchants.description ILIKE ? OR merchants.name ILIKE ?",
			"%"+search+"%",
			"%"+search+"%",
		)
	}
	if request.Header.Get("ID-Company") != "" {
		stmt = stmt.Where("company_id = ?", request.Header.Get("ID-Company"))
	}
	if request.URL.Query().Get("status") != "" {
		stmt = stmt.Where("status = ?", request.URL.Query().Get("status"))
	}
	stmt = stmt.Model(&models.MerchantModel{})
	utils.FixRequest(&request)
	page := pg.With(stmt).Request(request).Response(&[]models.MerchantModel{})
	page.Page = page.Page + 1
	items := page.Items.(*[]models.MerchantModel)
	newItems := make([]models.MerchantModel, 0)

	for _, v := range *items {
		if v.CompanyID != nil {
			var company models.CompanyModel
			err := s.db.Select("name", "id").Where("id = ?", v.CompanyID).First(&company).Error
			if err == nil {
				v.Company = &company
			}
		}
		newItems = append(newItems, v)

	}
	page.Items = &newItems
	return page, nil
}

func (s *MerchantService) GetMerchantProducts(request http.Request, search string, merchantID string, warehouseID *string) (paginate.Page, error) {
	pg := paginate.New()
	var products []models.ProductModel

	stmt := s.db.Joins("JOIN product_merchants ON product_merchants.product_model_id = products.id").
		Joins("JOIN brands ON brands.id = products.brand_id").
		Where("product_merchants.merchant_model_id = ?", merchantID)

	if search != "" {
		stmt = stmt.Where("products.name ILIKE ? OR products.sku ILIKE ? OR products.description ILIKE ? OR brands.name ILIKE ?",
			"%"+search+"%",
			"%"+search+"%",
			"%"+search+"%",
			"%"+search+"%")
	}
	stmt = stmt.Select("products.*", "product_merchants.price as price").Model(&models.ProductModel{})

	utils.FixRequest(&request)
	page := pg.With(stmt).Request(request).Response(&products)
	page.Page = page.Page + 1

	items := page.Items.(*[]models.ProductModel)
	newItems := make([]models.ProductModel, 0)

	for _, v := range *items {
		if warehouseID != nil {
			totalStock, _ := s.ctx.InventoryService.(*inventory.InventoryService).StockMovementService.GetCurrentStock(v.ID, *warehouseID)
			v.TotalStock = totalStock

		}

		newItems = append(newItems, v)
	}
	page.Items = &newItems

	return page, nil
}

func (s *MerchantService) CountMerchantByStatus(status string) (int64, error) {

	var count int64
	if err := s.db.Model(&models.MerchantModel{}).Where("status = ?", status).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *MerchantService) AddProductsToMerchant(merchantID string, productIDs []string) error {

	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, productID := range productIDs {
		var product models.ProductModel
		if err := tx.Select("id", "price").Where("id = ?", productID).First(&product).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.ProductMerchant{}).Where("product_model_id = ? AND merchant_model_id = ?", productID, merchantID).FirstOrCreate(&models.ProductMerchant{
			ProductModelID:  productID,
			MerchantModelID: merchantID,
			Price:           product.Price,
		}).Error; err != nil {
			tx.Rollback()
			return err
		}

	}

	return tx.Commit().Error
}

func (s *MerchantService) DeleteProductsFromMerchant(merchantID string, productIDs []string) error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Where("merchant_model_id = ? AND product_model_id IN (?)", merchantID, productIDs).
		Delete(&models.ProductMerchant{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *MerchantService) EditProductPrice(merchantID, productID string, price float64) error {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Model(&models.ProductMerchant{}).Where("product_model_id = ? AND merchant_model_id = ?", productID, merchantID).
		Updates(map[string]interface{}{
			"price": price,
		}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
