package finance

import (
	"fmt"
	"log"

	"github.com/AMETORY/ametory-erp-modules/context"
	"github.com/AMETORY/ametory-erp-modules/finance/account"
	"github.com/AMETORY/ametory-erp-modules/finance/asset"
	"github.com/AMETORY/ametory-erp-modules/finance/bank"
	"github.com/AMETORY/ametory-erp-modules/finance/journal"
	"github.com/AMETORY/ametory-erp-modules/finance/report"
	"github.com/AMETORY/ametory-erp-modules/finance/tax"
	"github.com/AMETORY/ametory-erp-modules/finance/transaction"
	"gorm.io/gorm"
)

type FinanceService struct {
	ctx                *context.ERPContext
	AccountService     *account.AccountService
	TransactionService *transaction.TransactionService
	BankService        *bank.BankService
	JournalService     *journal.JournalService
	ReportService      *report.FinanceReportService
	TaxService         *tax.TaxService
	AssetService       *asset.AssetService
}

func NewFinanceService(ctx *context.ERPContext) *FinanceService {
	fmt.Println("INIT FINANCE SERVICE")
	var service = FinanceService{
		ctx: ctx,
	}
	service.AccountService = account.NewAccountService(ctx.DB, ctx)
	service.TransactionService = transaction.NewTransactionService(ctx.DB, ctx, service.AccountService)
	service.BankService = bank.NewBankService(ctx.DB, ctx)
	service.JournalService = journal.NewJournalService(ctx.DB, ctx, service.AccountService, service.TransactionService)
	service.ReportService = report.NewFinanceReportService(ctx.DB, ctx, service.AccountService, service.TransactionService)
	service.TaxService = tax.NewTaxService(ctx.DB, ctx, service.AccountService)
	service.AssetService = asset.NewAssetService(ctx.DB, ctx)
	err := service.Migrate()
	if err != nil {
		panic(err)
	}
	return &service
}

func (s *FinanceService) Migrate() error {
	if s.ctx.SkipMigration {
		return nil
	}
	if err := account.Migrate(s.ctx.DB); err != nil {
		log.Println("ERROR ACCOUNT MIGRATE", err)
		return err
	}
	if err := transaction.Migrate(s.ctx.DB); err != nil {
		log.Println("ERROR TRANSACTION MIGRATE", err)
		return err
	}
	if err := journal.Migrate(s.ctx.DB); err != nil {
		log.Println("ERROR JOURNAL MIGRATE", err)
		return err
	}
	if err := tax.Migrate(s.ctx.DB); err != nil {
		log.Println("ERROR TAX MIGRATE", err)
		return err
	}
	if err := report.Migrate(s.ctx.DB); err != nil {
		log.Println("ERROR REPORT MIGRATE", err)
		return err
	}
	if err := asset.Migrate(s.ctx.DB); err != nil {
		log.Println("ERROR ASSET MIGRATE", err)
		return err
	}
	// if err := transaction.Migrate(s.TransactionService.DB()); err != nil {
	// 	return err
	// }
	// if err := invoice.Migrate(s.InvoiceService.DB()); err != nil {
	// 	return err
	// }
	return nil
}
func (s *FinanceService) DB() *gorm.DB {
	return s.ctx.DB
}
