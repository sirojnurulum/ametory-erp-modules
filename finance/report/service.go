package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/AMETORY/ametory-erp-modules/contact"
	"github.com/AMETORY/ametory-erp-modules/context"
	"github.com/AMETORY/ametory-erp-modules/finance/account"
	"github.com/AMETORY/ametory-erp-modules/finance/transaction"
	"github.com/AMETORY/ametory-erp-modules/shared"
	"github.com/AMETORY/ametory-erp-modules/shared/constants"
	"github.com/AMETORY/ametory-erp-modules/shared/models"
	"github.com/AMETORY/ametory-erp-modules/utils"
	"github.com/morkid/paginate"
	"gorm.io/gorm"
)

type FinanceReportService struct {
	db                 *gorm.DB
	ctx                *context.ERPContext
	accountService     *account.AccountService
	transactionService *transaction.TransactionService
	contactService     *contact.ContactService
}

func NewFinanceReportService(db *gorm.DB, ctx *context.ERPContext, accountService *account.AccountService, transactionService *transaction.TransactionService) *FinanceReportService {
	return &FinanceReportService{
		db:                 db,
		ctx:                ctx,
		accountService:     accountService,
		transactionService: transactionService,
	}
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.ClosingBook{})
}

func (s *FinanceReportService) SetContactService(contactService *contact.ContactService) {
	s.contactService = contactService
}
func (s *FinanceReportService) GenerateProfitLoss(report *models.ProfitLoss) error {

	return nil
}

func (s *FinanceReportService) GenerateAccountReport(accountID string, companyID *string, request http.Request) (*models.AccountReport, error) {
	account, err := s.accountService.GetAccountByID(accountID)
	if err != nil {
		return nil, err
	}
	if request.URL.Query().Get("start_date") == "" {
		return nil, fmt.Errorf("start date is required")
	}
	if request.URL.Query().Get("end_date") == "" {
		return nil, fmt.Errorf("end date is required")
	}

	// fmt.Println(account)

	// var companyID *string
	// if request.Header.Get("ID-Company") != "" {
	// 	compID := request.Header.Get("ID-Company")
	// 	companyID = &compID
	// }
	var startDate, endDate *time.Time
	startDateParsed, err := time.Parse("2006-01-02", request.URL.Query().Get("start_date"))
	if err != nil {
		return nil, err
	}
	endDateParsed, err := time.Parse("2006-01-02", request.URL.Query().Get("end_date"))
	if err != nil {
		return nil, err
	}
	startDate = &startDateParsed
	endDate = &endDateParsed

	var balanceCurrent, balanceBefore float64
	// BEFORE
	debit, credit, _ := s.GetAccountBalance(accountID, companyID, nil, startDate)
	switch account.Type {
	case models.EXPENSE, models.COST, models.CONTRA_LIABILITY, models.CONTRA_EQUITY, models.CONTRA_REVENUE, models.RECEIVABLE:
		balanceCurrent = debit - credit
	case models.LIABILITY, models.EQUITY, models.REVENUE, models.INCOME, models.CONTRA_ASSET, models.CONTRA_EXPENSE:
		balanceCurrent = credit - debit
	case models.ASSET:
		balanceCurrent = debit - credit
	}
	balanceBefore = balanceCurrent

	// CURRENT
	pageCurrent, err := s.GetAccountTransactions(accountID, companyID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	balance, _, _ := s.getBalance(&pageCurrent, &balanceCurrent, companyID)

	var balanceAfter float64
	// AFTER
	debit, credit, _ = s.GetAccountBalance(accountID, companyID, endDate, nil)
	switch account.Type {
	case models.EXPENSE, models.COST, models.CONTRA_LIABILITY, models.CONTRA_EQUITY, models.CONTRA_REVENUE, models.RECEIVABLE:
		balanceAfter = debit - credit
	case models.LIABILITY, models.EQUITY, models.REVENUE, models.INCOME, models.CONTRA_ASSET, models.CONTRA_EXPENSE:
		balanceAfter = credit - debit
	case models.ASSET:
		balanceAfter = debit - credit
	}

	return &models.AccountReport{
		StartDate:      startDate,
		EndDate:        endDate,
		Account:        *account,
		BalanceBefore:  balanceBefore,
		TotalBalance:   balanceBefore + balance + balanceAfter,
		CurrentBalance: balance,
		Transactions:   pageCurrent,
	}, nil
}
func (s *FinanceReportService) getBalance(page *[]models.TransactionModel, currentBalance *float64, companyID *string) (float64, float64, float64) {

	newItems := make([]models.TransactionModel, 0)
	var balance, credit, debit float64
	for _, item := range *page {
		if item.TransactionRefID != nil {
			if item.TransactionRefType == "journal" {
				var journalRef models.JournalModel
				err := s.db.Where("id = ?", item.TransactionRefID).Where("company_id = ?", *companyID).First(&journalRef).Error
				if err == nil {
					item.JournalRef = &journalRef
				}
			}
			if item.TransactionRefType == "transaction" {
				var transRef models.TransactionModel
				err := s.db.Preload("Account").Where("id = ?", item.TransactionRefID).Where("company_id = ?", *companyID).First(&transRef).Error
				if err == nil {
					item.TransactionRef = &transRef
				}
			}
			if item.TransactionRefType == "sales" {
				var salesRef models.SalesModel
				err := s.db.Where("id = ?", item.TransactionRefID).Where("company_id = ?", *companyID).First(&salesRef).Error
				if err == nil {
					item.SalesRef = &salesRef
				}
			}
			if item.TransactionSecondaryRefType == "sales" {
				var salesRef models.SalesModel
				err := s.db.Where("id = ?", item.TransactionSecondaryRefID).Where("company_id = ?", *companyID).First(&salesRef).Error
				if err == nil {
					item.SalesRef = &salesRef
				}
			}
			if item.TransactionRefType == "purchase" {
				var purchaseRef models.PurchaseOrderModel
				err := s.db.Where("id = ?", item.TransactionRefID).Where("company_id = ?", *companyID).First(&purchaseRef).Error
				if err == nil {
					item.PurchaseRef = &purchaseRef
				}
			}
			if item.TransactionSecondaryRefType == "purchase" {
				var purchaseRef models.PurchaseOrderModel
				err := s.db.Where("id = ?", item.TransactionSecondaryRefID).Where("company_id = ?", *companyID).First(&purchaseRef).Error
				if err == nil {
					item.PurchaseRef = &purchaseRef
				}
			}
			if item.TransactionRefType == "net-surplus" {
				var netSurplusRef models.NetSurplusModel
				err := s.db.Where("id = ?", item.TransactionRefID).Where("company_id = ?", *companyID).First(&netSurplusRef).Error
				if err == nil {
					item.NetSurplusRef = &netSurplusRef
				}
			}
		}
		curBalance := s.getBalanceAmount(item)
		balance += curBalance
		// fmt.Printf("balance %f, currentBalance %f\n", balance, *currentBalance)
		*currentBalance += curBalance
		item.Balance = *currentBalance
		credit += item.Credit
		debit += item.Debit

		newItems = append(newItems, item)
	}

	*page = newItems

	return balance, credit, debit
}

func (s *FinanceReportService) GetAccountBalance(accountID string, companyID *string, startDate *time.Time, endDate *time.Time) (float64, float64, error) {
	amount := struct {
		Credit float64 `sql:"credit"`
		Debit  float64 `sql:"debit"`
	}{}
	db := s.db.Model(&models.TransactionModel{}).Select("sum(credit) as credit, sum(debit) as debit").Where("account_id = ?", accountID)
	if startDate != nil {
		db = db.Where("date >= ?", startDate)
	}
	if endDate != nil {
		db = db.Where("date < ?", endDate)
	}
	if companyID != nil {
		db = db.Where("company_id", *companyID)
	}
	err := db.Scan(&amount).Error
	if err != nil {
		return 0, 0, err
	}

	return amount.Debit, amount.Credit, nil
}

func (s *FinanceReportService) GetAccountTransactions(accountID string, companyID *string, startDate *time.Time, endDate *time.Time) ([]models.TransactionModel, error) {
	var transactions []models.TransactionModel
	db := s.db.Preload("Account").Select("transactions.*, accounts.name as account_name").Joins("LEFT JOIN accounts ON accounts.id = transactions.account_id")

	if startDate != nil {
		db = db.Where("transactions.date >= ?", *startDate)
	}
	if endDate != nil {
		db = db.Where("transactions.date < ?", *endDate)
	}
	db = db.Where("transactions.account_id = ?", accountID)
	if companyID != nil {
		db = db.Where("transactions.company_id = ?", *companyID)
	}
	db = db.Order("date asc")
	err := db.Find(&transactions).Error
	if err != nil {
		return nil, err
	}

	return transactions, nil

}

func (s *FinanceReportService) getBalanceAmount(transaction models.TransactionModel) float64 {
	switch transaction.Account.Type {
	case models.EXPENSE, models.COST, models.CONTRA_LIABILITY, models.CONTRA_EQUITY, models.CONTRA_REVENUE, models.RECEIVABLE:
		return transaction.Debit - transaction.Credit
	case models.LIABILITY, models.EQUITY, models.REVENUE, models.INCOME, models.CONTRA_ASSET, models.CONTRA_EXPENSE:
		return transaction.Credit - transaction.Debit
	case models.ASSET:
		return transaction.Debit - transaction.Credit
	}
	return 0
}

func (s *FinanceReportService) GenerateCogsReport(report models.GeneralReport) (*models.COGSReport, error) {
	var inventoryAccount models.AccountModel
	err := s.db.Where("is_inventory_account = ? and company_id = ?", true, report.CompanyID).First(&inventoryAccount).Error
	if err != nil {
		return nil, errors.New("inventory account not found")
	}
	fmt.Println("GET STOCK OPNAME ACCOUNT")
	var stockOpnameAccounts []models.AccountModel
	err = s.db.Where("is_stock_opname_account = ? and company_id = ?", true, report.CompanyID).Find(&stockOpnameAccounts).Error
	if err != nil {
		return nil, errors.New("inventory account not found")
	}

	stockOpnameAccountIDs := []string{}
	for _, v := range stockOpnameAccounts {
		stockOpnameAccountIDs = append(stockOpnameAccountIDs, v.ID)
	}

	var beginningInventory, purchases, freightInAndOtherCost, totalPurchases, purchaseReturns, purchaseDiscounts, totalPurchaseDiscounts, netPurchases, goodsAvailable, endingInventory, cogs, stockOpname float64
	amount := struct {
		Sum float64 `sql:"sum"`
	}{}
	err = s.db.Model(&models.TransactionModel{}).
		Where("date < ?", report.StartDate).
		Select("sum(debit-credit) as sum").
		Where("account_id = ?", inventoryAccount.ID).
		Where("company_id = ?", report.CompanyID).
		Scan(&amount).Error
	if err != nil {
		return nil, err
	}
	beginningInventory = amount.Sum

	err = s.db.Model(&models.TransactionModel{}).
		Where("is_purchase_cost = ?", false).
		Where("is_purchase = ?", true).
		Where("debit > ?", 0).
		Where("date between ? and ?", report.StartDate, report.EndDate).
		Select("sum(debit-credit) as sum").
		Where("account_id = ?", inventoryAccount.ID).
		Where("company_id = ?", report.CompanyID).
		Scan(&amount).Error
	if err != nil {
		return nil, err
	}
	purchases = amount.Sum

	err = s.db.Model(&models.TransactionModel{}).
		Where("is_purchase_cost = ?", true).
		Where("debit > ?", 0).
		Where("date between ? and ?", report.StartDate, report.EndDate).
		Select("sum(debit-credit) as sum").
		Where("account_id = ?", inventoryAccount.ID).
		Where("company_id = ?", report.CompanyID).
		Scan(&amount).Error
	if err != nil {
		return nil, err
	}
	freightInAndOtherCost = amount.Sum
	totalPurchases = purchases + freightInAndOtherCost

	err = s.db.Model(&models.TransactionModel{}).
		Where("is_return = ?", true).
		Where("date between ? and ?", report.StartDate, report.EndDate).
		Select("sum(credit-debit) as sum").
		Where("account_id = ?", inventoryAccount.ID).
		Where("company_id = ?", report.CompanyID).
		Scan(&amount).Error
	if err != nil {
		return nil, err
	}
	purchaseReturns = amount.Sum
	err = s.db.Model(&models.TransactionModel{}).
		Where("is_discount = ?", true).
		Where("date between ? and ?", report.StartDate, report.EndDate).
		Select("sum(credit-debit) as sum").
		Where("account_id = ?", inventoryAccount.ID).
		Where("company_id = ?", report.CompanyID).
		Scan(&amount).Error
	if err != nil {
		return nil, err
	}
	purchaseDiscounts = amount.Sum

	totalPurchaseDiscounts = purchaseReturns + purchaseDiscounts

	err = s.db.Model(&models.TransactionModel{}).
		Where("date < ?", report.EndDate).
		Select("sum(debit-credit) as sum").
		Where("account_id = ?", inventoryAccount.ID).
		Where("company_id = ?", report.CompanyID).
		Scan(&amount).Error
	if err != nil {
		return nil, err
	}
	endingInventory = amount.Sum

	// STOCK OPNAME

	fmt.Println("GET STOCK OPNAME")
	err = s.db.Model(&models.TransactionModel{}).
		Where("date < ?", report.EndDate).
		Select("sum(debit-credit) as sum").
		Where("account_id IN (?)", stockOpnameAccountIDs).
		Where("company_id = ?", report.CompanyID).
		Scan(&amount).Error
	if err != nil {
		return nil, err
	}
	stockOpname = amount.Sum

	netPurchases = totalPurchases - totalPurchaseDiscounts
	goodsAvailable = beginningInventory + netPurchases
	cogs = goodsAvailable - endingInventory - stockOpname

	cogsData := models.COGSReport{
		BeginningInventory:     beginningInventory,
		Purchases:              purchases,
		FreightInAndOtherCost:  freightInAndOtherCost,
		TotalPurchases:         totalPurchases,
		PurchaseReturns:        purchaseReturns,
		PurchaseDiscounts:      purchaseDiscounts,
		TotalPurchaseDiscounts: totalPurchaseDiscounts,
		NetPurchases:           netPurchases,
		GoodsAvailable:         goodsAvailable,
		EndingInventory:        endingInventory,
		COGS:                   cogs,
		InventoryAccount:       inventoryAccount,
		StockOpname:            stockOpname,
	}
	cogsData.StartDate = report.StartDate
	cogsData.EndDate = report.EndDate
	// utils.LogJson(cogsData)
	return &cogsData, nil
}

func (s *FinanceReportService) CreateClosingBook(closingBook *models.ClosingBook) error {
	return s.db.Create(closingBook).Error
}

func (s *FinanceReportService) GetClosingBookByID(closingBookID string) (*models.ClosingBook, error) {
	var closingBook models.ClosingBook
	err := s.db.Where("id = ?", closingBookID).First(&closingBook).Error
	if err != nil {
		return nil, err
	}
	return &closingBook, nil
}

func (s *FinanceReportService) GetClosingBook(request http.Request, search string) (paginate.Page, error) {
	pg := paginate.New()
	stmt := s.db
	if search != "" {
		stmt = stmt.Where("notes ILIKE ?",
			"%"+search+"%",
		)
	}
	if request.Header.Get("ID-Company") != "" {
		stmt = stmt.Where("company_id = ? or company_id is null", request.Header.Get("ID-Company"))
	}

	if request.URL.Query().Get("status") != "" {
		stmt = stmt.Where("status = ?", request.URL.Query().Get("status"))
	}
	request.URL.Query().Get("page")
	stmt = stmt.Model(&models.ClosingBook{})
	utils.FixRequest(&request)
	page := pg.With(stmt).Request(request).Response(&[]models.ClosingBook{})
	page.Page = page.Page + 1
	return page, nil
}

func (s *FinanceReportService) DeleteClosingBook(closingBookID string) error {
	err := s.db.Where("transaction_secondary_ref_id = ?", closingBookID).Unscoped().Delete(&models.TransactionModel{}).Error
	if err != nil {
		return err
	}
	err = s.db.Where("id = ?", closingBookID).Delete(&models.ClosingBook{}).Error
	if err != nil {
		return err
	}
	return nil
}

func (s *FinanceReportService) GenerateClosingBook(
	closingBook *models.ClosingBook,
	cashflowGroupSetting *models.CashflowGroupSetting,
	userID string,
	description string,
	retainingID string,
	profitSumID string,
	taxPayableID *string,
	taxExpenseID *string,
	taxPercentage float64,
) error {
	if cashflowGroupSetting == nil {
		return errors.New("cashflow group setting is required")
	}

	var transactions []models.TransactionModel
	// 🧾 Langkah 1: Menutup Akun Pendapatan
	err := s.db.Where("transaction_secondary_ref_id = ?", closingBook.ID).Unscoped().Delete(&models.TransactionModel{}).Error
	if err != nil {
		return err
	}

	closingBook.ProfitLossData = nil
	closingBook.CashFlowData = nil
	closingBook.BalanceSheetData = nil
	closingBook.TrialBalanceData = nil
	closingBook.CapitalChangeData = nil
	closingBook.TransactionData = nil

	// CLOSING BOOK RETAIN EARNING

	report := models.GeneralReport{
		CompanyID: *closingBook.CompanyID,
		StartDate: closingBook.StartDate,
		EndDate:   closingBook.EndDate,
	}
	var cogsClosingAccount models.AccountModel
	err = s.db.Where("is_cogs_closing_account = ? and company_id = ? AND name = ?", true, closingBook.CompanyID, "HARGA POKOK PENJUALAN").First(&cogsClosingAccount).Error
	if err != nil {
		return errors.New("cogs account not found")
	}

	profitLoss, err := s.GenerateProfitLossReport(report)
	if err != nil {
		return err
	}
	now := time.Now()
	closingBookID := closingBook.ID
	err = s.db.Transaction(func(tx *gorm.DB) error {
		summary := models.ClosingSummary{}
		// Pengakuan Beban Pajak dan Kewajiban Pajak:
		taxTotal := 0.0
		netProfit := profitLoss.NetProfit
		if taxPayableID != nil && taxExpenseID != nil && taxPercentage > 0 {
			taxTotal = profitLoss.NetProfit * taxPercentage / 100
			netProfit -= taxTotal
		}
		summary.IncomeTax = taxTotal
		summary.TaxPercentage = taxPercentage
		summary.NetIncome = netProfit
		summary.TotalIncome = profitLoss.GrossProfit
		summary.TotalExpense = profitLoss.TotalExpense

		// fmt.Println("profitLoss.NetProfit", profitLoss.NetProfit)
		// fmt.Println("taxTotal", taxTotal)
		// fmt.Println("taxPercentage", taxPercentage)
		// fmt.Println("taxPayableID", *taxPayableID)
		// fmt.Println("taxExpenseID", *taxExpenseID)

		// saleAmount := 0.0
		// 1. Tutup akun Pendapatan
		totalIncome := 0.0
		for _, v := range profitLoss.Profit {
			if !v.IsCogs {
				if v.Sum != 0 {
					var debit, credit float64
					if v.Sum > 0 {
						debit = math.Abs(v.Sum)
						credit = 0
					} else {
						credit = math.Abs(v.Sum)
						debit = 0
					}
					itransID := utils.Uuid()
					itransID2 := utils.Uuid()
					incomeTrans := models.TransactionModel{
						BaseModel:                   shared.BaseModel{ID: itransID},
						Code:                        utils.RandString(10, false),
						Date:                        now,
						UserID:                      &userID,
						CompanyID:                   closingBook.CompanyID,
						Credit:                      utils.AmountRound(credit, 2),
						Debit:                       utils.AmountRound(debit, 2),
						Amount:                      utils.AmountRound(math.Abs(v.Sum), 2),
						Description:                 v.Name,
						AccountID:                   &v.ID,
						TransactionRefID:            &itransID2,
						TransactionRefType:          "transaction",
						TransactionSecondaryRefID:   &closingBookID,
						TransactionSecondaryRefType: "closing-book",
						Notes:                       "Tutup akun Pendapatan / Retur Penjualan",
						// IsNetSurplus:       true,
					}
					transactions = append(transactions, incomeTrans)
					err = tx.Create(&incomeTrans).Error
					if err != nil {
						return err
					}
					incomeTrans2 := models.TransactionModel{
						BaseModel:                   shared.BaseModel{ID: itransID2},
						Code:                        incomeTrans.Code,
						Date:                        now,
						UserID:                      &userID,
						CompanyID:                   closingBook.CompanyID,
						Credit:                      utils.AmountRound(incomeTrans.Debit, 2),
						Debit:                       utils.AmountRound(incomeTrans.Credit, 2),
						Amount:                      utils.AmountRound(math.Abs(math.Abs(v.Sum)), 2),
						Description:                 "Ikhtisar Laba Rugi",
						AccountID:                   &profitSumID,
						TransactionRefID:            &itransID,
						TransactionRefType:          "transaction",
						TransactionSecondaryRefID:   &closingBookID,
						TransactionSecondaryRefType: "closing-book",
						Notes:                       "Tutup akun Pendapatan / Retur Penjualan",
						// IsNetSurplus:       true,
					}
					transactions = append(transactions, incomeTrans2)
					err = tx.Create(&incomeTrans2).Error
					if err != nil {
						return err
					}

					// saleAmount += v.Sum
				}
			}
			totalIncome += v.Sum
		}

		// 2. Tutup akun Retur Penjualan
		for _, v := range profitLoss.Profit {
			if v.IsCogs {
				if v.Sum != 0 {
					var debit, credit float64
					if v.Sum > 0 {
						debit = math.Abs(v.Sum)
						credit = 0
					} else {
						credit = math.Abs(v.Sum)
						debit = 0
					}
					cogsTransID := utils.Uuid()
					cogsTransID2 := utils.Uuid()
					cogs := models.TransactionModel{
						BaseModel: shared.BaseModel{ID: cogsTransID},

						Code:                        utils.RandString(10, false),
						Date:                        now,
						UserID:                      &userID,
						CompanyID:                   closingBook.CompanyID,
						Credit:                      utils.AmountRound(credit, 2),
						Debit:                       utils.AmountRound(debit, 2),
						Amount:                      utils.AmountRound(math.Abs(v.Sum), 2),
						Description:                 v.Name,
						AccountID:                   &cogsClosingAccount.ID,
						TransactionRefID:            &cogsTransID2,
						TransactionRefType:          "transaction",
						TransactionSecondaryRefID:   &closingBookID,
						TransactionSecondaryRefType: "closing-book",
						Notes:                       "Tutup akun HPP",
						// IsNetSurplus:       true,
					}
					transactions = append(transactions, cogs)
					err = tx.Create(&cogs).Error
					if err != nil {
						return err
					}
					cogs2 := models.TransactionModel{
						BaseModel:                   shared.BaseModel{ID: cogsTransID2},
						Code:                        cogs.Code,
						Date:                        now,
						UserID:                      &userID,
						CompanyID:                   closingBook.CompanyID,
						Credit:                      utils.AmountRound(cogs.Debit, 2),
						Debit:                       utils.AmountRound(cogs.Credit, 2),
						Amount:                      utils.AmountRound(math.Abs(v.Sum), 2),
						Description:                 "Ikhtisar Laba Rugi",
						AccountID:                   &profitSumID,
						TransactionRefID:            &cogsTransID,
						TransactionRefType:          "transaction",
						TransactionSecondaryRefID:   &closingBookID,
						TransactionSecondaryRefType: "closing-book",
						Notes:                       "Tutup akun HPP",
						// IsNetSurplus:       true,
					}
					transactions = append(transactions, cogs2)
					err = tx.Create(&cogs2).Error
					if err != nil {
						return err
					}
				}
			}
			totalIncome += v.Sum
		}

		// summary.TotalIncome = totalIncome

		totalExpense := 0.0
		for _, v := range profitLoss.Loss {
			if v.Sum != 0 {
				var debit, credit float64
				if v.Sum > 0 {
					debit = math.Abs(v.Sum)
					credit = 0
				} else {
					credit = math.Abs(v.Sum)
					debit = 0
				}
				lossTransID := utils.Uuid()
				lossTransID2 := utils.Uuid()

				loss2 := models.TransactionModel{
					BaseModel:                   shared.BaseModel{ID: lossTransID},
					Code:                        utils.RandString(10, false),
					Date:                        now,
					UserID:                      &userID,
					CompanyID:                   closingBook.CompanyID,
					Credit:                      utils.AmountRound(credit, 2),
					Debit:                       utils.AmountRound(debit, 2),
					Amount:                      utils.AmountRound(math.Abs(v.Sum), 2),
					Description:                 "Ikhtisar Laba Rugi",
					AccountID:                   &profitSumID,
					TransactionRefID:            &lossTransID2,
					TransactionRefType:          "transaction",
					TransactionSecondaryRefID:   &closingBookID,
					TransactionSecondaryRefType: "closing-book",
					Notes:                       "Tutup akun Beban Operasional",
					// IsNetSurplus:       true,
				}
				transactions = append(transactions, loss2)
				err = tx.Create(&loss2).Error
				if err != nil {
					return err
				}
				loss := models.TransactionModel{
					BaseModel:                   shared.BaseModel{ID: lossTransID2},
					Code:                        loss2.Code,
					Date:                        now,
					UserID:                      &userID,
					CompanyID:                   closingBook.CompanyID,
					Credit:                      utils.AmountRound(loss2.Debit, 2),
					Debit:                       utils.AmountRound(loss2.Credit, 2),
					Amount:                      utils.AmountRound(math.Abs(v.Sum), 2),
					Description:                 v.Name,
					AccountID:                   &v.ID,
					TransactionRefID:            &lossTransID,
					TransactionRefType:          "transaction",
					TransactionSecondaryRefID:   &closingBookID,
					TransactionSecondaryRefType: "closing-book",
					Notes:                       "Tutup akun Beban Operasional",
					// IsNetSurplus:       true,
				}
				transactions = append(transactions, loss)
				err = tx.Create(&loss).Error
				if err != nil {
					return err
				}

			}
			totalExpense += v.Sum
		}
		// summary.TotalExpense = totalExpense
		if taxPayableID != nil && taxExpenseID != nil && taxPercentage > 0 {
			summary.TaxExpenseID = taxExpenseID
			summary.TaxPayableID = taxPayableID
			taxTransID := utils.Uuid()
			taxTransID2 := utils.Uuid()
			taxTransID3 := utils.Uuid()
			taxTransID4 := utils.Uuid()

			tax := models.TransactionModel{
				BaseModel:                   shared.BaseModel{ID: taxTransID},
				Code:                        utils.RandString(10, false),
				Date:                        now,
				UserID:                      &userID,
				CompanyID:                   closingBook.CompanyID,
				Debit:                       utils.AmountRound(taxTotal, 2),
				Amount:                      utils.AmountRound(math.Abs(taxTotal), 2),
				Description:                 "Beban Pajak Penghasilan",
				AccountID:                   taxExpenseID,
				TransactionRefID:            &taxTransID2,
				TransactionRefType:          "transaction",
				TransactionSecondaryRefID:   &closingBookID,
				TransactionSecondaryRefType: "closing-book",
				Notes:                       "Pengakuan Beban Pajak Penghasilan Badan",
				// IsNetSurplus:       true,
			}
			transactions = append(transactions, tax)
			err = tx.Create(&tax).Error
			if err != nil {
				return err
			}

			tax2 := models.TransactionModel{
				BaseModel:                   shared.BaseModel{ID: taxTransID2},
				Code:                        tax.Code,
				Date:                        now,
				UserID:                      &userID,
				CompanyID:                   closingBook.CompanyID,
				Credit:                      utils.AmountRound(taxTotal, 2),
				Amount:                      utils.AmountRound(math.Abs(taxTotal), 2),
				Description:                 "Utang Pajak Penghasilan",
				AccountID:                   taxPayableID,
				TransactionRefID:            &taxTransID,
				TransactionRefType:          "transaction",
				TransactionSecondaryRefID:   &closingBookID,
				TransactionSecondaryRefType: "closing-book",
				Notes:                       "Pengakuan Beban Pajak Penghasilan Badan",
				// IsNetSurplus:       true,
			}
			transactions = append(transactions, tax2)
			err = tx.Create(&tax2).Error
			if err != nil {
				return err
			}

			tax3 := models.TransactionModel{
				BaseModel:                   shared.BaseModel{ID: taxTransID3},
				Code:                        tax.Code,
				Date:                        now,
				UserID:                      &userID,
				CompanyID:                   closingBook.CompanyID,
				Debit:                       utils.AmountRound(taxTotal, 2),
				Amount:                      utils.AmountRound(math.Abs(taxTotal), 2),
				Description:                 "Ikhtisar Laba Rugi",
				AccountID:                   &profitSumID,
				TransactionRefID:            &taxTransID4,
				TransactionRefType:          "transaction",
				TransactionSecondaryRefID:   &closingBookID,
				TransactionSecondaryRefType: "closing-book",
				Notes:                       "Tutup akun Beban Pajak",
				// IsNetSurplus:       true,
			}
			transactions = append(transactions, tax3)
			err = tx.Create(&tax3).Error
			if err != nil {
				return err
			}
			tax4 := models.TransactionModel{
				BaseModel:                   shared.BaseModel{ID: taxTransID4},
				Code:                        tax.Code,
				Date:                        now,
				UserID:                      &userID,
				CompanyID:                   closingBook.CompanyID,
				Credit:                      utils.AmountRound(taxTotal, 2),
				Amount:                      utils.AmountRound(math.Abs(taxTotal), 2),
				Description:                 "Beban Pajak Penghasilan",
				AccountID:                   taxExpenseID,
				TransactionRefID:            &taxTransID3,
				TransactionRefType:          "transaction",
				TransactionSecondaryRefID:   &closingBookID,
				TransactionSecondaryRefType: "closing-book",
				Notes:                       "Tutup akun Beban Pajak",
				// IsNetSurplus:       true,
			}
			transactions = append(transactions, tax4)
			err = tx.Create(&tax4).Error
			if err != nil {
				return err
			}
		}

		closingTransID := utils.Uuid()
		closingTransID2 := utils.Uuid()
		closingTrans2 := models.TransactionModel{
			BaseModel:                   shared.BaseModel{ID: closingTransID},
			Code:                        utils.RandString(10, false),
			Date:                        now,
			UserID:                      &userID,
			CompanyID:                   closingBook.CompanyID,
			Debit:                       utils.AmountRound(profitLoss.NetProfit-taxTotal, 2),
			Amount:                      utils.AmountRound(math.Abs(profitLoss.NetProfit-taxTotal), 2),
			Description:                 "Ikhtisar Laba Rugi",
			AccountID:                   &profitSumID,
			TransactionRefID:            &closingTransID2,
			TransactionRefType:          "transaction",
			TransactionSecondaryRefID:   &closingBookID,
			TransactionSecondaryRefType: "closing-book",
			Notes:                       "Tutup saldo akhir Ikhtisar Laba Rugi ke Laba Ditahan",
			// IsNetSurplus:       true,
		}
		transactions = append(transactions, closingTrans2)
		err = tx.Create(&closingTrans2).Error
		if err != nil {
			return err
		}

		closingTrans := models.TransactionModel{
			BaseModel:                   shared.BaseModel{ID: closingTransID2},
			Code:                        closingTrans2.Code,
			Date:                        now,
			UserID:                      &userID,
			CompanyID:                   closingBook.CompanyID,
			Credit:                      utils.AmountRound(profitLoss.NetProfit-taxTotal, 2),
			Amount:                      utils.AmountRound(math.Abs(profitLoss.NetProfit-taxTotal), 2),
			Description:                 "Laba Ditahan / SHU Tahun Berjalan",
			AccountID:                   &retainingID,
			TransactionRefID:            &closingTransID,
			TransactionRefType:          "transaction",
			TransactionSecondaryRefID:   &closingBookID,
			TransactionSecondaryRefType: "closing-book",
			Notes:                       "Tutup saldo akhir Ikhtisar Laba Rugi ke Laba Ditahan",
			// IsNetSurplus:       true,
		}
		summary.EarningRetainID = &retainingID
		transactions = append(transactions, closingTrans)
		err = tx.Create(&closingTrans).Error
		if err != nil {
			return err
		}

		closingBook.Status = "RELEASED"
		closingSummaryByte, _ := json.Marshal(summary)
		closingSummaryStr := string(closingSummaryByte)
		closingBook.ClosingSummaryData = &closingSummaryStr

		err = tx.Where("id = ?", closingBook.ID).Updates(closingBook).Error
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}

	var closingData models.ClosingBook
	err = s.db.Where("id = ?", closingBookID).First(&closingData).Error
	if err != nil {
		return err
	}
	closingData.Status = "RELEASED"
	cashflow, err := s.GenerateCashFlowReport(models.CashFlowReport{
		GeneralReport: report,
		Operating:     cashflowGroupSetting.Operating,
		Financing:     cashflowGroupSetting.Financing,
		Investing:     cashflowGroupSetting.Investing,
	})
	if err != nil {
		return err
	}

	cashflowByte, _ := json.Marshal(cashflow)
	cashflowStr := string(cashflowByte)

	// profitLoss, err = s.GenerateProfitLossReport(report)
	// if err != nil {
	// 	return err
	// }
	plbyte, _ := json.Marshal(profitLoss)
	profitLossStr := string(plbyte)

	balanceSheet, err := s.GenerateBalanceSheet(report)
	if err != nil {
		return err
	}

	balanceSheetByte, _ := json.Marshal(balanceSheet)
	balanceSheetStr := string(balanceSheetByte)

	trialBalance, err := s.TrialBalanceReport(report)
	if err != nil {
		return err
	}
	capitalChange, err := s.GenerateCapitalChangeReport(report)
	if err != nil {
		return err
	}

	profitLoss, err = s.GenerateProfitLossReport(report)
	if err != nil {
		return err
	}
	capitalChangeByte, _ := json.Marshal(capitalChange)
	capitalChangeStr := string(capitalChangeByte)

	trialBalanceByte, _ := json.Marshal(trialBalance)
	trialBalanceStr := string(trialBalanceByte)

	transactionByte, _ := json.Marshal(transactions)
	transactionStr := string(transactionByte)

	closingData.ProfitLossData = &profitLossStr
	closingData.CashFlowData = &cashflowStr
	closingData.BalanceSheetData = &balanceSheetStr
	closingData.TrialBalanceData = &trialBalanceStr
	closingData.CapitalChangeData = &capitalChangeStr
	closingData.TransactionData = &transactionStr
	err = s.db.Where("id = ?", closingBookID).Updates(closingData).Error
	if err != nil {
		return err
	}

	return nil
}
func (s *FinanceReportService) TrialBalanceReport(report models.GeneralReport) (*models.TrialBalanceReport, error) {
	var trialBalanceReport models.TrialBalanceReport = models.TrialBalanceReport{
		CompanyID: &report.CompanyID,
		StartDate: report.StartDate,
		EndDate:   report.EndDate,
	}
	types := []models.AccountType{
		models.ASSET,
		models.LIABILITY,
		models.EQUITY,
		models.REVENUE,
		models.EXPENSE,
		models.COST,
		models.RECEIVABLE,
		models.CONTRA_REVENUE,
	}

	for _, v := range types {
		var accounts []models.AccountModel
		s.db.Model(&models.AccountModel{}).Where("type = ? AND company_id = ?", v, report.CompanyID).
			// Where("is_cogs_closing_account = ? OR is_profit_loss_closing_account = ?", false, false).
			Find(&accounts)
		for _, account := range accounts {
			if account.IsCogsClosingAccount {
				continue
			}
			// TRIAL BALANCE
			trialBalanceDebit, trialBalanceCredit, err := s.GetAccountBalance(account.ID, &report.CompanyID, &report.EndDate, nil)
			if err != nil {
				return nil, err
			}
			trialBalanceReport.TrialBalance = append(trialBalanceReport.TrialBalance, models.TrialBalanceRow{
				ID:      account.ID,
				Debit:   trialBalanceDebit,
				Credit:  trialBalanceCredit,
				Name:    account.Name,
				Code:    account.Code,
				Balance: s.getTransBalance(&account, trialBalanceDebit, trialBalanceCredit),
			})

			// ADJUSTMENT
			adjustmentDebit, adjustmentCredit, err := s.GetAccountBalance(account.ID, &report.CompanyID, &report.StartDate, &report.EndDate)
			if err != nil {
				return nil, err
			}

			trialBalanceReport.Adjustment = append(trialBalanceReport.Adjustment, models.TrialBalanceRow{
				ID:      account.ID,
				Debit:   adjustmentDebit,
				Credit:  adjustmentCredit,
				Name:    account.Name,
				Code:    account.Code,
				Balance: s.getTransBalance(&account, adjustmentDebit, adjustmentCredit),
			})

			// BALANCE SHEET
			balanceSheetDebit, balanceSheetCredit, err := s.GetAccountBalance(account.ID, &report.CompanyID, nil, &report.EndDate)
			if err != nil {
				return nil, err
			}

			trialBalanceReport.BalanceSheet = append(trialBalanceReport.BalanceSheet, models.TrialBalanceRow{
				ID:      account.ID,
				Debit:   balanceSheetDebit,
				Credit:  balanceSheetCredit,
				Name:    account.Name,
				Code:    account.Code,
				Balance: s.getTransBalance(&account, balanceSheetDebit, balanceSheetCredit),
			})

		}

	}
	return &trialBalanceReport, nil
}
func (s *FinanceReportService) GenerateProfitLossReport(report models.GeneralReport) (*models.ProfitLossReport, error) {
	profitLoss := models.ProfitLossReport{}
	cogsReport, err := s.GenerateCogsReport(report)
	if err != nil {
		return nil, err
	}

	fmt.Println("GENERATE PROFIT LOSS", report.StartDate, report.EndDate)

	revenueAccounts := []models.AccountModel{}
	err = s.db.Where("type IN (?)", []models.AccountType{models.INCOME, models.REVENUE, models.CONTRA_REVENUE}).Where("company_id = ?", report.CompanyID).Find(&revenueAccounts).Error
	if err != nil {
		return nil, err
	}
	revenueSum := 0.0
	for _, revenue := range revenueAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err = s.db.Model(&models.TransactionModel{}).
			Where("date between ? and ?", report.StartDate, report.EndDate).
			Select("sum(credit-debit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", revenue.ID).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}
		profitLoss.Profit = append(profitLoss.Profit, models.ProfitLossAccount{
			ID:   revenue.ID,
			Name: revenue.Name,
			Code: revenue.Code,
			Sum:  amount.Sum,
		})
		revenueSum += amount.Sum
	}

	profitLoss.Profit = append(profitLoss.Profit, models.ProfitLossAccount{
		Name:   "Harga Pokok Penjualan",
		Sum:    -cogsReport.COGS,
		Link:   "/cogs",
		IsCogs: true,
	})

	profitLoss.GrossProfit = revenueSum - cogsReport.COGS

	expenseAccounts := []models.AccountModel{}
	err = s.db.Where("type IN (?)", []models.AccountType{models.EXPENSE}).Where("company_id = ?", report.CompanyID).Find(&expenseAccounts).Error
	if err != nil {
		return nil, err
	}
	expenseSum := 0.0
	for _, expense := range expenseAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err = s.db.Model(&models.TransactionModel{}).
			Where("date between ? and ?", report.StartDate, report.EndDate).
			Select("sum(debit-credit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", expense.ID).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}
		profitLoss.Loss = append(profitLoss.Loss, models.ProfitLossAccount{
			ID:   expense.ID,
			Name: expense.Name,
			Code: expense.Code,
			Sum:  amount.Sum,
		})
		expenseSum += amount.Sum
	}

	profitLoss.TotalExpense += expenseSum

	// NET SURPLUS
	// equityAccounts := []models.AccountModel{}
	// err = s.db.Where("type IN (?) AND is_profit_loss_account = ?", []models.AccountType{models.EQUITY}, true).Find(&equityAccounts).Error
	// if err != nil {
	// 	return nil, err
	// }
	// netSurplus := 0.0
	// for _, equity := range equityAccounts {
	// 	amount := struct {
	// 		Sum float64 `sql:"sum"`
	// 	}{}
	// 	err = s.db.Model(&models.TransactionModel{}).
	// 		Where("date between ? and ?", report.StartDate, report.EndDate).
	// 		Select("sum(debit-credit) as sum").
	// 		Joins("JOIN accounts ON accounts.id = transactions.account_id").
	// 		Where("transactions.account_id = ?", equity.ID).
	// 		Scan(&amount).Error
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	profitLoss.NetSurplus = append(profitLoss.NetSurplus, models.ProfitLossAccount{
	// 		ID:   equity.ID,
	// 		Name: equity.Name,
	// 		Code: equity.Code,
	// 		Sum:  amount.Sum,
	// 	})
	// 	netSurplus += amount.Sum
	// 	fmt.Printf("%s => %f", equity.Name, amount.Sum)
	// }

	// profitLoss.TotalNetSurplus = netSurplus

	profitLoss.NetProfit = profitLoss.GrossProfit - profitLoss.TotalExpense
	return &profitLoss, nil
}

func (s *FinanceReportService) GenerateBalanceSheet(report models.GeneralReport) (*models.BalanceSheet, error) {
	balanceSheet := models.BalanceSheet{}
	balanceSheet.StartDate = report.StartDate
	balanceSheet.EndDate = report.EndDate

	// ASSETS
	// FIXED ACCOUNT
	fixedAccounts := []models.AccountModel{}
	err := s.db.Where("type IN (?) AND cashflow_group = ? AND company_id = ?", []string{string(models.ASSET), string(models.CONTRA_ASSET)}, "fixed_asset", report.CompanyID).Find(&fixedAccounts).Error
	if err != nil {
		return nil, errors.New("fixedAccounts account not found")
	}
	fixedAmount := 0.0
	for _, expense := range fixedAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err = s.db.Model(&models.TransactionModel{}).
			Where("date <  ?", report.EndDate).
			Select("sum(debit-credit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", expense.ID).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}
		balanceSheet.FixedAssets = append(balanceSheet.FixedAssets, models.BalanceSheetAccount{
			ID:   expense.ID,
			Name: expense.Name,
			Code: expense.Code,
			Sum:  amount.Sum,
		})
		fixedAmount += amount.Sum
	}
	balanceSheet.TotalFixed = fixedAmount

	// CURRENT ACCOUNT
	currentAccounts := []models.AccountModel{}
	err = s.db.Where("type = ? AND cashflow_group = ? AND company_id = ?", "ASSET", "current_asset", report.CompanyID).Find(&currentAccounts).Error
	if err != nil {

	}
	currentAmount := 0.0
	for _, expense := range currentAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err = s.db.Model(&models.TransactionModel{}).
			Where("date <  ?", report.EndDate).
			Select("sum(debit-credit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", expense.ID).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}
		balanceSheet.CurrentAssets = append(balanceSheet.CurrentAssets, models.BalanceSheetAccount{
			ID:   expense.ID,
			Name: expense.Name,
			Code: expense.Code,
			Sum:  amount.Sum,
		})
		currentAmount += amount.Sum
	}

	// RECEIVABLE ACCOUNT
	receivableAccounts := []models.AccountModel{}
	err = s.db.Where("type = ?  AND company_id = ?", "RECEIVABLE", report.CompanyID).Find(&receivableAccounts).Error
	if err != nil {
		return nil, errors.New("receivableAccounts account not found")
	}

	for _, expense := range receivableAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err = s.db.Model(&models.TransactionModel{}).
			Where("date <  ?", report.EndDate).
			Select("sum(debit-credit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", expense.ID).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}
		balanceSheet.CurrentAssets = append(balanceSheet.CurrentAssets, models.BalanceSheetAccount{
			ID:   expense.ID,
			Name: expense.Name,
			Code: expense.Code,
			Sum:  amount.Sum,
		})
		currentAmount += amount.Sum
	}

	// INVENTORY
	report.StartDate = time.Time{}
	cogsReport, err := s.GenerateCogsReport(report)
	if err != nil {
		return nil, err
	}
	balanceSheet.CurrentAssets = append(balanceSheet.CurrentAssets, models.BalanceSheetAccount{
		ID:   cogsReport.InventoryAccount.ID,
		Code: cogsReport.InventoryAccount.Code,
		Name: cogsReport.InventoryAccount.Name,
		Sum:  cogsReport.EndingInventory,
	})

	currentAmount += cogsReport.EndingInventory
	balanceSheet.TotalCurrent = currentAmount

	balanceSheet.TotalAssets = balanceSheet.TotalFixed + balanceSheet.TotalCurrent
	// LIABILITY AND EQUITY

	// LIABILITY ACCOUNT
	liabilityAccounts := []models.AccountModel{}
	err = s.db.Where("type = ?  AND company_id = ?", "LIABILITY", report.CompanyID).Find(&liabilityAccounts).Error
	if err != nil {
		return nil, errors.New("liabilityAccounts account not found")
	}
	liabilityAmount := 0.0
	for _, expense := range liabilityAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err = s.db.Model(&models.TransactionModel{}).
			Where("date <  ?", report.EndDate).
			Select("sum(credit-debit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", expense.ID).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}
		balanceSheet.LiableAssets = append(balanceSheet.LiableAssets, models.BalanceSheetAccount{
			ID:   expense.ID,
			Name: expense.Name,
			Code: expense.Code,
			Sum:  amount.Sum,
		})
		liabilityAmount += amount.Sum
	}

	balanceSheet.TotalLiability = liabilityAmount

	// EQUITY ACCOUNT
	equityAccounts := []models.AccountModel{}
	err = s.db.Where("type = ?  AND company_id = ?", "EQUITY", report.CompanyID).Find(&equityAccounts).Error
	if err != nil {
		return nil, errors.New("equityAccounts account not found")
	}
	equityAmount := 0.0
	for _, expense := range equityAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err = s.db.Model(&models.TransactionModel{}).
			Where("date <  ?", report.EndDate).
			Select("sum(credit-debit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", expense.ID).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}
		balanceSheet.Equity = append(balanceSheet.Equity, models.BalanceSheetAccount{
			ID:   expense.ID,
			Name: expense.Name,
			Code: expense.Code,
			Sum:  amount.Sum,
		})
		equityAmount += amount.Sum
	}

	profitLoss, err := s.GenerateProfitLossReport(report)
	if err != nil {
		return nil, err
	}

	// PROFIT AND LOSS
	balanceSheet.Equity = append(balanceSheet.Equity, models.BalanceSheetAccount{
		Name: "Laba Ditahan",
		Sum:  profitLoss.NetProfit,
		Link: "/profit-loss-statement",
	})
	if len(profitLoss.NetSurplus) > 0 {
		balanceSheet.Equity = append(balanceSheet.Equity, models.BalanceSheetAccount{
			Name: "SHU Dibagikan",
			Sum:  -profitLoss.TotalNetSurplus,
			ID:   profitLoss.NetSurplus[len(profitLoss.NetSurplus)-1].ID,
		})
	}
	equityAmount += profitLoss.NetProfit - profitLoss.TotalNetSurplus
	balanceSheet.TotalEquity = equityAmount
	balanceSheet.TotalLiabilitiesAndEquity = balanceSheet.TotalLiability + balanceSheet.TotalEquity

	return &balanceSheet, nil
}

func (s *FinanceReportService) GenerateCapitalChangeReport(report models.GeneralReport) (*models.CapitalChangeReport, error) {
	capitalChange := models.CapitalChangeReport{}
	equityAccounts := []models.AccountModel{}
	err := s.db.Where("type = ?  AND company_id = ?", "EQUITY", report.CompanyID).Find(&equityAccounts).Error
	if err != nil {
		return nil, errors.New("equityAccounts account not found")
	}
	// Opening Balance
	openingBalance := 0.0
	for _, v := range equityAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err := s.db.Model(&models.TransactionModel{}).
			Where("date <  ?", report.EndDate).
			Select("sum(credit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", v.ID).
			Where("is_opening_balance = ?", true).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}

		openingBalance += amount.Sum
	}
	profitLoss, err := s.GenerateProfitLossReport(report)
	if err != nil {
		return nil, err
	}

	profitLossBalance := profitLoss.NetProfit

	privedBalance := 0.0
	for _, v := range equityAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err := s.db.Model(&models.TransactionModel{}).
			Where("date <  ?", report.EndDate).
			Select("sum(debit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", v.ID).
			Where("is_opening_balance = ?", false).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}

		privedBalance += amount.Sum
	}

	capitalChangeBalance := 0.0
	for _, v := range equityAccounts {
		amount := struct {
			Sum float64 `sql:"sum"`
		}{}
		err := s.db.Model(&models.TransactionModel{}).
			Where("date <  ?", report.EndDate).
			Select("sum(credit) as sum").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Where("transactions.account_id = ?", v.ID).
			Where("is_opening_balance = ?", false).
			Where("transactions.company_id = ?", report.CompanyID).
			Scan(&amount).Error
		if err != nil {
			return nil, err
		}

		capitalChangeBalance += amount.Sum
	}

	// amount := struct {
	// 	Sum float64 `sql:"sum"`
	// }{}
	// err := s.db.Model(&models.TransactionModel{}).
	// 	Where("date <  ?", report.StartDate).
	// 	Select("sum(credit-debit) as sum").
	// 	Joins("JOIN accounts ON accounts.id = transactions.account_id").
	// 	Where("accounts.type IN (?)", []models.AccountType{models.EQUITY}).
	// 	Scan(&amount).Error
	// if err != nil {
	// 	return nil, err
	// }

	capitalChange.OpeningBalance = openingBalance
	capitalChange.ProfitLoss = profitLossBalance
	capitalChange.PrivedBalance = -privedBalance
	capitalChange.CapitalChangeBalance = capitalChangeBalance
	capitalChange.EndingBalance = openingBalance + profitLossBalance + capitalChangeBalance - privedBalance
	return &capitalChange, nil
}

func (s *FinanceReportService) GenerateCashFlowReport(cashFlow models.CashFlowReport) (*models.CashFlowReport, error) {

	// utils.LogJson(cashFlow)

	fmt.Println("======================================")
	fmt.Println("OPERATING")
	fmt.Println("======================================")
	operating, totalOperating := s.getCashFlowAmount(cashFlow.Operating, &cashFlow.CompanyID)
	cashFlow.Operating = operating
	cashFlow.TotalOperating = totalOperating

	fmt.Println("======================================")
	fmt.Println("INVESTING")
	fmt.Println("======================================")
	investing, totalInvesting := s.getCashFlowAmount(cashFlow.Investing, &cashFlow.CompanyID)
	cashFlow.Investing = investing
	cashFlow.TotalInvesting = totalInvesting

	fmt.Println("======================================")
	fmt.Println("FINANCING")
	fmt.Println("======================================")
	financing, totalInvesting := s.getCashFlowAmount(cashFlow.Financing, &cashFlow.CompanyID)
	cashFlow.Financing = financing
	cashFlow.TotalFinancing = totalInvesting

	return &cashFlow, nil
}

func (s *FinanceReportService) getCashFlowAmount(groups []models.CashflowSubGroup, companyID *string) ([]models.CashflowSubGroup, float64) {

	total := 0.0
	for i, v := range groups {
		var transactions []models.TransactionModel
		s.db.Model(&transactions).
			Distinct("transRef.id refid, (transRef.debit - transRef.credit) amount, accountRef.name description").
			Joins("JOIN accounts ON accounts.id = transactions.account_id").
			Joins("JOIN transactions transRef ON transRef.id = transactions.transaction_ref_id").
			Joins("JOIN accounts accountRef ON accountRef.id = transRef.account_id").
			Where("accounts.cashflow_sub_group = ?", v.Name).
			Where("accounts.company_id = ?", *companyID).
			Where("accountRef.cashflow_sub_group = ?", "cash_bank").
			Group("refid, transactions.id, accountRef.name").
			Find(&transactions)

		amount := 0.0
		for _, t := range transactions {
			fmt.Printf("[%s] %s %f\n", v.Name, t.Description, t.Amount)
			amount += t.Amount
		}
		v.Amount = amount
		groups[i] = v
		total += amount
	}
	return groups, total
}

func (s *FinanceReportService) getTransBalance(account *models.AccountModel, debit, credit float64) float64 {
	switch account.Type {
	case models.EXPENSE, models.COST, models.CONTRA_LIABILITY, models.CONTRA_EQUITY, models.CONTRA_REVENUE, models.RECEIVABLE:
		return debit - credit
	case models.LIABILITY, models.EQUITY, models.REVENUE, models.INCOME, models.CONTRA_ASSET, models.CONTRA_EXPENSE:
		return credit - debit
	case models.ASSET:
		return debit - credit
	}
	return 0
}

func (s *FinanceReportService) GetMonthlySalesReport(companyID string, year int) ([]models.MonthlySalesReport, error) {
	var reports []models.MonthlySalesReport
	for month := 1; month <= 12; month++ {
		report := models.MonthlySalesReport{
			Year:    year,
			Month:   month,
			Total:   0,
			Company: companyID,
		}

		err := s.db.Raw(`
			SELECT
				COALESCE(SUM(total), 0) AS total_sales
			FROM
				sales
			WHERE
				sales.document_type = 'INVOICE'
				AND EXTRACT(YEAR FROM sales_date) = ?
				AND EXTRACT(MONTH FROM sales_date) = ?
				AND sales.company_id = ?
				AND sales.deleted_at is null
		`, year, month, companyID).Scan(&report.Total).Error
		if err != nil {
			return nil, err
		}
		report.MonthName = time.Month(month).String()[:3]
		reports = append(reports, report)
	}
	return reports, nil
}
func (s *FinanceReportService) GetWeeklySalesReport(companyID string, year, month int) ([]models.MonthlySalesReport, error) {
	var reports []models.MonthlySalesReport
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	for day := firstDay; day.Month() == firstDay.Month(); day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Monday {
			_, week := day.ISOWeek()
			report := models.MonthlySalesReport{
				Year:    year,
				Month:   month,
				Total:   0,
				Company: companyID,
			}

			err := s.db.Raw(`
				SELECT
					COALESCE(SUM(total), 0) AS total_sales
				FROM
					sales
				WHERE
					sales.document_type = 'INVOICE'
					AND EXTRACT(YEAR FROM sales_date) = ?
					AND EXTRACT(MONTH FROM sales_date) = ?
					AND EXTRACT(WEEK FROM sales_date) = ?
					AND sales.company_id = ?
					AND sales.deleted_at is null
			`, year, month, week, companyID).Scan(&report.Total).Error
			if err != nil {
				return nil, err
			}
			report.WeekName = fmt.Sprintf("Week %d", week)
			reports = append(reports, report)
		}
	}

	return reports, nil
}

func (s *FinanceReportService) GetWeeklyPurchaseReport(companyID string, year, month int) ([]models.MonthlySalesReport, error) {
	var reports []models.MonthlySalesReport
	firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	for day := firstDay; day.Month() == firstDay.Month(); day = day.AddDate(0, 0, 1) {
		if day.Weekday() == time.Monday {
			_, week := day.ISOWeek()
			report := models.MonthlySalesReport{
				Year:    year,
				Month:   month,
				Total:   0,
				Company: companyID,
			}

			err := s.db.Raw(`
				SELECT
					COALESCE(SUM(total), 0) AS total_sales
				FROM
					purchase_orders
				WHERE
					purchase_orders.document_type = 'BILL'
					AND EXTRACT(YEAR FROM purchase_date) = ?
					AND EXTRACT(MONTH FROM purchase_date) = ?
					AND EXTRACT(WEEK FROM purchase_date) = ?
					AND purchase_orders.company_id = ?
					AND purchase_orders.deleted_at is null
			`, year, month, week, companyID).Scan(&report.Total).Error
			if err != nil {
				return nil, err
			}
			report.WeekName = fmt.Sprintf("Week %d", week)
			reports = append(reports, report)
		}
	}

	return reports, nil
}
func (s *FinanceReportService) GetMonthlyPurchaseReport(companyID string, year int) ([]models.MonthlySalesReport, error) {
	var reports []models.MonthlySalesReport
	for month := 1; month <= 12; month++ {
		report := models.MonthlySalesReport{
			Year:    year,
			Month:   month,
			Total:   0,
			Company: companyID,
		}

		err := s.db.Raw(`
			SELECT
				COALESCE(SUM(total), 0) AS total_sales
			FROM
				purchase_orders
			WHERE
				purchase_orders.document_type = 'BILL'
				AND EXTRACT(YEAR FROM purchase_date) = ?
				AND EXTRACT(MONTH FROM purchase_date) = ?
				AND purchase_orders.company_id = ?
				AND purchase_orders.deleted_at is null
		`, year, month, companyID).Scan(&report.Total).Error
		if err != nil {
			return nil, err
		}
		report.MonthName = time.Month(month).String()[:3]
		reports = append(reports, report)
	}
	return reports, nil
}

func (s *FinanceReportService) CalculateSalesByTimeRange(
	companyID string,
	documentType string,
	timeRange string,
) (float64, error) {
	var total float64
	var err error

	switch timeRange {
	case "Q1":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(sales.total), 0) AS total
			FROM
				sales
			WHERE
				sales.company_id = ?
				AND sales.document_type = ?
				AND EXTRACT(QUARTER FROM sales_date) = 1
				AND EXTRACT(YEAR FROM sales_date) = EXTRACT(YEAR FROM CURRENT_DATE)
				AND sales.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "Q2":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(sales.total), 0) AS total
			FROM
				sales
			WHERE
				sales.company_id = ?
				AND sales.document_type = ?
				AND EXTRACT(QUARTER FROM sales_date) = 2
				AND EXTRACT(YEAR FROM sales_date) = EXTRACT(YEAR FROM CURRENT_DATE)
				AND sales.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "Q3":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(sales.total), 0) AS total
			FROM
				sales
			WHERE
				sales.company_id = ?
				AND sales.document_type = ?
				AND EXTRACT(QUARTER FROM sales_date) = 3
				AND EXTRACT(YEAR FROM sales_date) = EXTRACT(YEAR FROM CURRENT_DATE)
				AND sales.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "Q4":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(sales.total), 0) AS total
			FROM
				sales
			WHERE
				sales.company_id = ?
				AND sales.document_type = ?
				AND EXTRACT(QUARTER FROM sales_date) = 4
				AND EXTRACT(YEAR FROM sales_date) = EXTRACT(YEAR FROM CURRENT_DATE)
				AND sales.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "THIS_MONTH":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(sales.total), 0) AS total
			FROM
				sales
			WHERE
				sales.company_id = ?
				AND sales.document_type = ?
				AND sales_date BETWEEN date_trunc('month', CURRENT_DATE) AND date_trunc('month', CURRENT_DATE) + INTERVAL '1 month'
				AND sales.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "THIS_WEEK":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(sales.total), 0) AS total
			FROM
				sales
			WHERE
				sales.company_id = ?
				AND sales.document_type = ?
				AND sales_date BETWEEN date_trunc('week', CURRENT_DATE) AND date_trunc('week', CURRENT_DATE) + INTERVAL '1 week'
				AND sales.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "THIS_YEAR":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(sales.total), 0) AS total
			FROM
				sales
			WHERE
				sales.company_id = ?
				AND sales.document_type = ?
				AND sales_date BETWEEN date_trunc('year', CURRENT_DATE) AND date_trunc('year', CURRENT_DATE) + INTERVAL '1 year'
				AND sales.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	default:
		err = errors.New("invalid time range")
	}
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (s *FinanceReportService) CalculatePurchaseByTimeRange(
	companyID string,
	documentType string,
	timeRange string,
) (float64, error) {
	var total float64
	var err error

	switch timeRange {
	case "Q1":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(purchase_orders.total), 0) AS total
			FROM
				purchase_orders
			WHERE
				purchase_orders.company_id = ?
				AND purchase_orders.document_type = ?
				AND EXTRACT(QUARTER FROM purchase_date) = 1
				AND EXTRACT(YEAR FROM purchase_date) = EXTRACT(YEAR FROM CURRENT_DATE)
				AND purchase_orders.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "Q2":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(purchase_orders.total), 0) AS total
			FROM
				purchase_orders
			WHERE
				purchase_orders.company_id = ?
				AND purchase_orders.document_type = ?
				AND EXTRACT(QUARTER FROM purchase_date) = 2
				AND EXTRACT(YEAR FROM purchase_date) = EXTRACT(YEAR FROM CURRENT_DATE)
				AND purchase_orders.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "Q3":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(purchase_orders.total), 0) AS total
			FROM
				purchase_orders
			WHERE
				purchase_orders.company_id = ?
				AND purchase_orders.document_type = ?
				AND EXTRACT(QUARTER FROM purchase_date) = 3
				AND EXTRACT(YEAR FROM purchase_date) = EXTRACT(YEAR FROM CURRENT_DATE)
				AND purchase_orders.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "Q4":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(purchase_orders.total), 0) AS total
			FROM
				purchase_orders
			WHERE
				purchase_orders.company_id = ?
				AND purchase_orders.document_type = ?
				AND EXTRACT(QUARTER FROM purchase_date) = 4
				AND EXTRACT(YEAR FROM purchase_date) = EXTRACT(YEAR FROM CURRENT_DATE)
				AND purchase_orders.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "THIS_MONTH":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(purchase_orders.total), 0) AS total
			FROM
				purchase_orders
			WHERE
				purchase_orders.company_id = ?
				AND purchase_orders.document_type = ?
				AND purchase_date BETWEEN date_trunc('month', CURRENT_DATE) AND date_trunc('month', CURRENT_DATE) + INTERVAL '1 month'
				AND purchase_orders.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "THIS_WEEK":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(purchase_orders.total), 0) AS total
			FROM
				purchase_orders
			WHERE
				purchase_orders.company_id = ?
				AND purchase_orders.document_type = ?
				AND purchase_date BETWEEN date_trunc('week', CURRENT_DATE) AND date_trunc('week', CURRENT_DATE) + INTERVAL '1 week'
				AND purchase_orders.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	case "THIS_YEAR":
		err = s.db.Raw(`
			SELECT
				COALESCE(SUM(purchase_orders.total), 0) AS total
			FROM
				purchase_orders
			WHERE
				purchase_orders.company_id = ?
				AND purchase_orders.document_type = ?
				AND purchase_date BETWEEN date_trunc('year', CURRENT_DATE) AND date_trunc('year', CURRENT_DATE) + INTERVAL '1 year'
				AND purchase_orders.deleted_at is null
		`, companyID, documentType).Scan(&total).Error
	default:
		err = errors.New("invalid time range")
	}
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (s *FinanceReportService) GetSumCashBank(companyID string) (float64, error) {
	var total float64
	err := s.db.Raw(`
		SELECT
			COALESCE(SUM(transactions.debit - transactions.credit), 0) AS total
		FROM
			transactions
		JOIN accounts ON transactions.account_id = accounts.id
		WHERE
			accounts.type = ? AND cashflow_sub_group = ?
			AND accounts.company_id = ? 
			AND transactions.deleted_at is null
	`, models.ASSET, constants.CASH_BANK, companyID).Scan(&total).Error
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (s *FinanceReportService) GetAlmostDueSales(companyID string, interval int) ([]models.SalesList, error) {
	reports := []models.SalesList{}
	err := s.db.Raw(fmt.Sprintf(`
		SELECT
			sales.id,
			sales_number as number,
			contacts.name as contact_name,
			sales.total AS total,
			(sales.total - sales.paid) AS balance,
			sales.due_date as due_date
		FROM
			sales
		LEFT JOIN contacts ON sales.contact_id = contacts.id
		WHERE
			sales.company_id = ?
			AND sales.document_type = ?
			AND (sales.due_date <= CURRENT_DATE + INTERVAL '%v'  DAY OR sales.due_date IS NULL)
			AND sales.paid <> sales.total
			AND sales.deleted_at is null
		ORDER BY
			sales.due_date ASC
	`, interval), companyID, models.INVOICE).Scan(&reports).Error
	if err != nil {
		return nil, err
	}
	return reports, nil
}
func (s *FinanceReportService) GetAlmostDuePurchase(companyID string, interval int) ([]models.SalesList, error) {
	reports := []models.SalesList{}
	err := s.db.Raw(fmt.Sprintf(`
		SELECT
			purchase_orders.id,
			purchase_number as number,
			contacts.name as contact_name,
			purchase_orders.total AS total,
			(purchase_orders.total - purchase_orders.paid) AS balance,
			purchase_orders.due_date as due_date
		FROM
			purchase_orders
		LEFT JOIN contacts ON purchase_orders.contact_id = contacts.id
		WHERE
			purchase_orders.company_id = ?
			AND purchase_orders.document_type = ?
			AND (purchase_orders.due_date <= CURRENT_DATE + INTERVAL '%v' DAY OR purchase_orders.due_date IS NULL)
			AND purchase_orders.paid <> purchase_orders.total
			AND purchase_orders.deleted_at is null
		ORDER BY
			purchase_orders.due_date ASC
	`, interval), companyID, models.BILL).Scan(&reports).Error
	if err != nil {
		return nil, err
	}
	return reports, nil
}

func (s *FinanceReportService) GetProductSalesCustomers(companyID string, startDate, endDate time.Time, productIDs []string, customerIDs []string) ([]models.ProductSalesCustomer, error) {
	reports := []models.ProductSalesCustomer{}
	conditions := []string{
		"sales.company_id = ?",
		"sales.document_type = ?",
		"sales_items.deleted_at is null",
		"products.deleted_at is null",
		"contacts.deleted_at is null",
		"sales.deleted_at is null",
		"units.deleted_at is null",
		"sales.sales_date BETWEEN ? and ?",
		"sales.status IN ('POSTED', 'FINISHED')",
		"sales_items.product_id IS NOT NULL AND sales_items.product_id <> ''",
	}

	args := []interface{}{companyID, models.INVOICE, startDate, endDate}

	if len(productIDs) > 0 {
		conditions = append(conditions, "products.id IN (?)")
		args = append(args, productIDs)
	}

	if len(customerIDs) > 0 {
		conditions = append(conditions, "contacts.id IN (?)")
		args = append(args, customerIDs)
	}

	query := fmt.Sprintf(`
		SELECT
			products.id as product_id,
			products.sku as product_code,
			contacts.code as contact_code,
			contacts.id as contact_id,
			products.name as product_name,
			contacts.name as contact_name,
			COUNT(sales.id) as quantity,
			units.name as unit_name,
			units.code as unit_code,
			SUM(sales_items.quantity * sales_items.unit_value) as total_quantity,
			SUM(sales_items.total) as total_price
		FROM
			sales
		LEFT JOIN sales_items on sales.id = sales_items.sales_id
		LEFT JOIN products on sales_items.product_id = products.id
		LEFT JOIN contacts on sales.contact_id = contacts.id
		LEFT JOIN product_units on products.id = product_units.product_model_id AND product_units.is_default = true
		LEFT JOIN units on product_units.unit_model_id = units.id
		WHERE
			%s
		GROUP BY
			products.id,
			products.sku,
			products.name,
			contacts.id,
			contacts.code,
			contacts.name,
			units.name,
			units.code
		ORDER BY
			products.name ASC,
			products.id ASC,
			contacts.id ASC,
			contacts.name ASC
	`, strings.Join(conditions, " AND "))

	err := s.db.Raw(query, args...).Scan(&reports).Error
	if err != nil {
		return nil, err
	}
	return reports, nil
}

func (s *FinanceReportService) GetAccountReceivableLedger(companyID string, contactID string, startDate, endDate time.Time) (*models.AccountReceivableLedgerReport, error) {
	if s.contactService == nil {
		return nil, errors.New("contact service is not initialized")
	}
	reports := []models.AccountReceivableLedger{}
	err := s.db.Table("transactions").
		Select("distinct transactions.id, transactions.description, transactions.\"date\", transactions.debit, transactions.credit, "+
			"case when s.id is not null then s.sales_number when ss.id is not null then ss.sales_number when sss.id is not null then sss.sales_number else null end ref, "+
			"case when transactions.transaction_secondary_ref_type != '' then transactions.transaction_secondary_ref_type else transactions.transaction_ref_type end ref_type, "+
			"case when s.id is not null then s.id when ss.id is not null then ss.id when sss.id is not null then sss.id else null end ref_id").
		Joins("join accounts a on a.id = transactions.account_id").
		Joins("left join sales s on s.id = transactions.transaction_ref_id").
		Joins("left join sales ss on ss.id = transactions.transaction_secondary_ref_id").
		Joins("left join \"returns\" r on r.id = transactions.transaction_secondary_ref_id").
		Joins("left join sales sss on sss.id = r.ref_id").
		Where("a.type in (?)", []string{"RECEIVABLE"}).
		Where("(transactions.transaction_ref_type = ? or transactions.transaction_secondary_ref_type = ? or transactions.transaction_secondary_ref_type = ?)",
			"sales", "sales", "return_sales").
		Where("(s.status in (?) or ss.status in (?) or sss.status in (?))",
			[]string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}).
		Where("(s.document_type = ? or ss.document_type = ? or sss.document_type = ?)",
			"INVOICE", "INVOICE", "INVOICE").
		Where("(s.contact_id = ? or ss.contact_id = ? or sss.contact_id = ?)", contactID, contactID, contactID).
		Where("transactions.company_id = ?", companyID).
		Where("transactions.date between ? and ?", startDate, endDate).
		Order("date asc").
		Scan(&reports).Error
	if err != nil {
		return nil, err
	}

	var totalBefore struct {
		TotalDebit  float64
		TotalCredit float64
		Balance     float64
	}

	err = s.db.Table("transactions").
		Select("sum(debit) as total_debit, sum(credit) as total_credit, sum(debit - credit) as balance").
		Joins("join accounts a on a.id = transactions.account_id").
		Joins("left join sales s on s.id = transactions.transaction_ref_id").
		Joins("left join sales ss on ss.id = transactions.transaction_secondary_ref_id").
		Joins("left join \"returns\" r on r.id = transactions.transaction_secondary_ref_id").
		Joins("left join sales sss on sss.id = r.ref_id").
		Where("a.type in (?)", []string{"RECEIVABLE"}).
		Where("(transactions.transaction_ref_type = ? or transactions.transaction_secondary_ref_type = ? or transactions.transaction_secondary_ref_type = ?)",
			"sales", "sales", "return_sales").
		Where("(s.status in (?) or ss.status in (?) or sss.status in (?))",
			[]string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}).
		Where("(s.document_type = ? or ss.document_type = ? or sss.document_type = ?)",
			"INVOICE", "INVOICE", "INVOICE").
		Where("(s.contact_id = ? or ss.contact_id = ? or sss.contact_id = ?)",
			contactID, contactID, contactID).
		Where("transactions.company_id = ?", companyID).Where("transactions.date < ?", startDate).Scan(&totalBefore).Error
	if err != nil {
		return nil, err
	}

	var totalAfter struct {
		TotalDebit  float64
		TotalCredit float64
		Balance     float64
	}

	err = s.db.Table("transactions").
		Select("sum(debit) as total_debit, sum(credit) as total_credit, sum(debit - credit) as balance").
		Joins("join accounts a on a.id = transactions.account_id").
		Joins("left join sales s on s.id = transactions.transaction_ref_id").
		Joins("left join sales ss on ss.id = transactions.transaction_secondary_ref_id").
		Joins("left join \"returns\" r on r.id = transactions.transaction_secondary_ref_id").
		Joins("left join sales sss on sss.id = r.ref_id").
		Where("a.type in (?)", []string{"RECEIVABLE"}).
		Where("(transactions.transaction_ref_type = ? or transactions.transaction_secondary_ref_type = ? or transactions.transaction_secondary_ref_type = ?)",
			"sales", "sales", "return_sales").
		Where("(s.status in (?) or ss.status in (?) or sss.status in (?))",
			[]string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}).
		Where("(s.document_type = ? or ss.document_type = ? or sss.document_type = ?)",
			"INVOICE", "INVOICE", "INVOICE").
		Where("(s.contact_id = ? or ss.contact_id = ? or sss.contact_id = ?)",
			contactID, contactID, contactID).
		Where("transactions.company_id = ?", companyID).Where("transactions.date > ?", endDate).Scan(&totalAfter).Error
	if err != nil {
		return nil, err
	}

	balance := totalBefore.Balance
	var totalDebit, totalCredit, totalBalance float64
	for i, v := range reports {
		balance += v.Debit - v.Credit
		v.Balance = balance
		reports[i] = v
		totalDebit += v.Debit
		totalCredit += v.Credit
		totalBalance += v.Debit - v.Credit
	}

	var contact models.ContactModel
	s.db.First(&contact, "id = ?", contactID)

	if contact.DebtLimit > 0 {
		contact.DebtLimitRemain = contact.DebtLimit - s.contactService.GetTotalDebt(&contact)
	}
	if contact.ReceivablesLimit > 0 {
		contact.ReceivablesLimitRemain = contact.ReceivablesLimit - s.contactService.GetTotalReceivable(&contact)
	}

	return &models.AccountReceivableLedgerReport{
		Ledgers:            reports,
		TotalDebit:         totalDebit,
		TotalCredit:        totalCredit,
		TotalBalance:       totalBalance,
		TotalDebitBefore:   totalBefore.TotalDebit,
		TotalCreditBefore:  totalBefore.TotalCredit,
		TotalBalanceBefore: totalBefore.Balance,
		TotalDebitAfter:    totalAfter.TotalDebit,
		TotalCreditAfter:   totalAfter.TotalCredit,
		TotalBalanceAfter:  totalAfter.Balance,
		GrandTotalDebit:    totalDebit + totalBefore.TotalDebit + totalAfter.TotalDebit,
		GrandTotalCredit:   totalCredit + totalBefore.TotalCredit + totalAfter.TotalCredit,
		GrandTotalBalance:  totalBalance + totalBefore.Balance + totalAfter.Balance,
		Contact:            contact,
	}, nil
}

func (s *FinanceReportService) GetAccountPayableLedger(companyID string, contactID string, startDate, endDate time.Time) (*models.AccountReceivableLedgerReport, error) {
	if s.contactService == nil {
		return nil, errors.New("contact service is not initialized")
	}
	reports := []models.AccountReceivableLedger{}
	err := s.db.Table("transactions").
		Select("distinct transactions.id, transactions.description, transactions.\"date\", transactions.debit, transactions.credit, "+
			"case when s.id is not null then s.purchase_number when ss.id is not null then ss.purchase_number when sss.id is not null then sss.purchase_number else null end ref, "+
			"case when transactions.transaction_secondary_ref_type != '' then transactions.transaction_secondary_ref_type else transactions.transaction_ref_type end ref_type, "+
			"case when s.id is not null then s.id when ss.id is not null then ss.id when sss.id is not null then sss.id else null end ref_id").
		Joins("join accounts a on a.id = transactions.account_id").
		Joins("left join purchase_orders s on s.id = transactions.transaction_ref_id").
		Joins("left join purchase_orders ss on ss.id = transactions.transaction_secondary_ref_id").
		Joins("left join \"returns\" r on r.id = transactions.transaction_secondary_ref_id").
		Joins("left join purchase_orders sss on sss.id = r.ref_id").
		Where("a.type in (?)", []string{"LIABILITY"}).
		Where("(transactions.transaction_ref_type = ? or transactions.transaction_secondary_ref_type = ? or transactions.transaction_secondary_ref_type = ?)",
			"purchase", "purchase", "return_purchase").
		Where("(s.status in (?) or ss.status in (?) or sss.status in (?))",
			[]string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}).
		Where("(s.document_type = ? or ss.document_type = ? or sss.document_type = ?)",
			"BILL", "BILL", "BILL").
		Where("(s.contact_id = ? or ss.contact_id = ? or sss.contact_id = ?)", contactID, contactID, contactID).
		Where("transactions.company_id = ?", companyID).
		Where("transactions.date between ? and ?", startDate, endDate).
		Order("date asc").
		Scan(&reports).Error
	if err != nil {
		return nil, err
	}

	var totalBefore struct {
		TotalDebit  float64
		TotalCredit float64
		Balance     float64
	}

	err = s.db.Table("transactions").
		Select("sum(debit) as total_debit, sum(credit) as total_credit, sum(debit - credit) as balance").
		Joins("join accounts a on a.id = transactions.account_id").
		Joins("left join purchase_orders s on s.id = transactions.transaction_ref_id").
		Joins("left join purchase_orders ss on ss.id = transactions.transaction_secondary_ref_id").
		Joins("left join \"returns\" r on r.id = transactions.transaction_secondary_ref_id").
		Joins("left join purchase_orders sss on sss.id = r.ref_id").
		Where("a.type in (?)", []string{"LIABILITY"}).
		Where("(transactions.transaction_ref_type = ? or transactions.transaction_secondary_ref_type = ? or transactions.transaction_secondary_ref_type = ?)",
			"purchase", "purchase", "return_purchase").
		Where("(s.status in (?) or ss.status in (?) or sss.status in (?))",
			[]string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}).
		Where("(s.document_type = ? or ss.document_type = ? or sss.document_type = ?)",
			"BILL", "BILL", "BILL").
		Where("(s.contact_id = ? or ss.contact_id = ? or sss.contact_id = ?)",
			contactID, contactID, contactID).
		Where("transactions.company_id = ?", companyID).Where("transactions.date < ?", startDate).Scan(&totalBefore).Error
	if err != nil {
		return nil, err
	}

	var totalAfter struct {
		TotalDebit  float64
		TotalCredit float64
		Balance     float64
	}

	err = s.db.Table("transactions").
		Select("sum(debit) as total_debit, sum(credit) as total_credit, sum(debit - credit) as balance").
		Joins("join accounts a on a.id = transactions.account_id").
		Joins("left join purchase_orders s on s.id = transactions.transaction_ref_id").
		Joins("left join purchase_orders ss on ss.id = transactions.transaction_secondary_ref_id").
		Joins("left join \"returns\" r on r.id = transactions.transaction_secondary_ref_id").
		Joins("left join purchase_orders sss on sss.id = r.ref_id").
		Where("a.type in (?)", []string{"LIABILITY"}).
		Where("(transactions.transaction_ref_type = ? or transactions.transaction_secondary_ref_type = ? or transactions.transaction_secondary_ref_type = ?)",
			"purchase", "purchase", "return_purchase").
		Where("(s.status in (?) or ss.status in (?) or sss.status in (?))",
			[]string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}, []string{"POSTED", "FINISHED"}).
		Where("(s.document_type = ? or ss.document_type = ? or sss.document_type = ?)",
			"BILL", "BILL", "BILL").
		Where("(s.contact_id = ? or ss.contact_id = ? or sss.contact_id = ?)",
			contactID, contactID, contactID).
		Where("transactions.company_id = ?", companyID).Where("transactions.date > ?", endDate).Scan(&totalAfter).Error
	if err != nil {
		return nil, err
	}

	balance := totalBefore.Balance
	var totalDebit, totalCredit, totalBalance float64
	for i, v := range reports {
		balance += v.Credit - v.Debit
		v.Balance = balance
		reports[i] = v
		totalDebit += v.Debit
		totalCredit += v.Credit
		totalBalance += v.Credit - v.Debit
	}

	var contact models.ContactModel
	s.db.First(&contact, "id = ?", contactID)

	if contact.DebtLimit > 0 {
		contact.DebtLimitRemain = contact.DebtLimit - s.contactService.GetTotalDebt(&contact)
	}
	if contact.ReceivablesLimit > 0 {
		contact.ReceivablesLimitRemain = contact.ReceivablesLimit - s.contactService.GetTotalReceivable(&contact)
	}

	return &models.AccountReceivableLedgerReport{
		Ledgers:            reports,
		TotalDebit:         totalDebit,
		TotalCredit:        totalCredit,
		TotalBalance:       totalBalance,
		TotalDebitBefore:   totalBefore.TotalDebit,
		TotalCreditBefore:  totalBefore.TotalCredit,
		TotalBalanceBefore: totalBefore.Balance,
		TotalDebitAfter:    totalAfter.TotalDebit,
		TotalCreditAfter:   totalAfter.TotalCredit,
		TotalBalanceAfter:  totalAfter.Balance,
		GrandTotalDebit:    totalDebit + totalBefore.TotalDebit + totalAfter.TotalDebit,
		GrandTotalCredit:   totalCredit + totalBefore.TotalCredit + totalAfter.TotalCredit,
		GrandTotalBalance:  totalBalance + totalBefore.Balance + totalAfter.Balance,
		Contact:            contact,
	}, nil
}
