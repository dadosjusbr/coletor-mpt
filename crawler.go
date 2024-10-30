package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"github.com/dadosjusbr/status"
)

type crawler struct {
	// Aqui temos os atributos e métodos necessários para realizar a coleta dos dados
	generalTimeout   time.Duration
	timeBetweenSteps time.Duration
	downloadTimeout  time.Duration
	year             string
	month            string
	output           string
}

func (c crawler) crawl() ([]string, error) {
	// Chromedp setup.
	log.SetOutput(os.Stderr) // Enviando logs para o stderr para não afetar a execução do coletor.
	alloc, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36"),
			chromedp.Flag("headless", true), // mude para false para executar com navegador visível.
			chromedp.NoSandbox,
			chromedp.DisableGPU,
		)...,
	)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(
		alloc,
		chromedp.WithLogf(log.Printf), // remover comentário para depurar
	)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, c.generalTimeout)
	defer cancel()

	// Contracheques
	log.Printf("Selecionando contracheques (%s/%s)...", c.month, c.year)
	if err := c.selecionaContracheque(ctx); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Seleção realizada com sucesso!\n")

	log.Printf("Realizando download (%s/%s)...", c.month, c.year)
	cqFname := c.downloadFilePath("contracheques")
	if err := c.exportaPlanilha(ctx, cqFname); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Download realizado com sucesso!\n")

	// Verbas indenizatórias
	log.Printf("Selecionando indenizações (%s/%s)...", c.month, c.year)
	if err := c.selecionaVerbas(ctx); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Seleção realizada com sucesso!\n")

	log.Printf("Realizando download (%s/%s)...", c.month, c.year)
	iFname := c.downloadFilePath("indenizacoes")
	if err := c.exportaPlanilha(ctx, iFname); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Download realizado com sucesso!\n")
	return []string{cqFname, iFname}, nil
}

func (c crawler) selecionaContracheque(ctx context.Context) error {
	return chromedp.Run(ctx,
		chromedp.EmulateViewport(1920, 1080),
		chromedp.Navigate("https://mpt.mp.br/MPTransparencia/pages/portal/remuneracaoMembrosAtivos.xhtml"),
		chromedp.Sleep(c.timeBetweenSteps),
		// Seleciona o ano
		chromedp.SetValue(`//*[@id="j_idt177"]`, c.year, chromedp.BySearch),
		chromedp.Sleep(c.timeBetweenSteps),
		// Consulta
		chromedp.Click(`//*[@id="j_idt180"]`, chromedp.BySearch, chromedp.NodeVisible),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}
func (c crawler) selecionaVerbas(ctx context.Context) error {
	return chromedp.Run(ctx,
		// // Clica na aba Contracheque
		chromedp.Click(`//*[@id="sm-contracheque"]`, chromedp.BySearch, chromedp.NodeReady),
		chromedp.Sleep(c.timeBetweenSteps),
		// Clica em Verbas Indenizatórias e Outras Remunerações Temporárias
		chromedp.Click(`//*[@id="j_idt130"]`, chromedp.BySearch, chromedp.NodeReady),
		chromedp.Sleep(c.timeBetweenSteps),
		// Seleciona o ano
		chromedp.SetValue(`//*[@id="j_idt183"]`, c.year, chromedp.BySearch, chromedp.NodeReady),
		chromedp.Sleep(c.timeBetweenSteps),
		// Consulta
		chromedp.Click(`//*[@id="consultaForm"]/div[2]/div/input`, chromedp.BySearch, chromedp.NodeVisible),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}

// Retorna os caminhos completos dos arquivos baixados.
func (c crawler) downloadFilePath(prefix string) string {
	// A extensão das planilhas de contracheque é XLS, enquanto a das indenizações são ODS
	if strings.Contains(prefix, "contracheques") {
		return filepath.Join(c.output, fmt.Sprintf("membros-ativos-%s-%s-%s.xls", prefix, c.month, c.year))
	} else {
		return filepath.Join(c.output, fmt.Sprintf("membros-ativos-%s-%s-%s.ods", prefix, c.month, c.year))
	}
}
func (c crawler) exportaPlanilha(ctx context.Context, fName string) error {
	months := map[string]int{
		"01": 0,
		"02": 1,
		"03": 2,
		"04": 3,
		"05": 4,
		"06": 5,
		"07": 6,
		"08": 7,
		"09": 8,
		"10": 9,
		"11": 10,
		"12": 11,
	}
	var selectMonth string
	// O XPath para o botão de download de contracheques e indenizações é diferente.
	if strings.Contains(fName, "contracheques") {
		selectMonth = fmt.Sprintf(`//*[@id="tabelaRemuneracao:%d:j_idt199"]/span`, months[c.month])
	} else {
		selectMonth = fmt.Sprintf(`//*[@id="tabelaMeses:%d:linkArq"]`, months[c.month])
	}
	tctx, tcancel := context.WithTimeout(ctx, 30*time.Second)
	defer tcancel()
	if err := chromedp.Run(tctx,
		// Altera o diretório de download
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(c.output).
			WithEventsEnabled(true),
		// Clica no botão de download do respectivo mês
		chromedp.Click(selectMonth, chromedp.BySearch, chromedp.NodeReady),
		chromedp.Sleep(c.downloadTimeout),
	); err != nil {
		return status.NewError(status.DataUnavailable, fmt.Errorf("não há dados disponíveis"))
	}
	if err := nomeiaDownload(c.output, fName); err != nil {
		status.ExitFromError(err)
	}
	if _, err := os.Stat(fName); os.IsNotExist(err) {
		return status.NewError(status.SystemError, fmt.Errorf("download do arquivo de %s não realizado", fName))
	}
	return nil
}
func nomeiaDownload(output, fName string) error {
	// Identifica qual foi o último arquivo
	files, err := os.ReadDir(output)
	if err != nil {
		return status.NewError(status.SystemError, fmt.Errorf("erro lendo diretório %s: %w", output, err))
	}
	var newestFPath string
	var newestTime int64 = 0
	for _, f := range files {
		fPath := filepath.Join(output, f.Name())
		fi, err := os.Stat(fPath)
		if err != nil {
			return status.NewError(status.SystemError, fmt.Errorf("erro obtendo informações sobre arquivo %s: %w", fPath, err))
		}
		currTime := fi.ModTime().Unix()
		if currTime > newestTime {
			newestTime = currTime
			newestFPath = fPath
		}
	}
	// Renomeia o último arquivo modificado.
	if err := os.Rename(newestFPath, fName); err != nil {
		return status.NewError(status.DataUnavailable, fmt.Errorf("não há dados disponíveis"))
	}
	return nil
}
