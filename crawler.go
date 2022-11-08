package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
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

	log.Printf("Realizando seleção - Contracheque (%s)...", c.year)
	if err := c.selecionaContracheque(ctx, c.year); err != nil {
		log.Fatalf("Erro no setup:%v", err)
	}
	log.Printf("Seleção realizada com sucesso!\n")

	log.Printf("Realizando download (%s)...", c.year)
	cqFname := c.downloadFilePath("contracheques")
	if err := c.exportaPlanilha(ctx, cqFname); err != nil {
		log.Fatalf("Erro no setup:%v", err)
	}
	log.Printf("Download realizado com sucesso!\n")
	log.Printf("Realizando seleção - Indenizações (%s)...", c.year)
	if err := c.selecionaVerbas(ctx, c.year); err != nil {
		log.Fatalf("Erro no setup:%v", err)
	}
	log.Printf("Seleção realizada com sucesso!\n")

	log.Printf("Realizando download (%s)...", c.year)
	iFname := c.downloadFilePath("indenizacoes")
	if err := c.exportaPlanilha(ctx, iFname); err != nil {
		log.Fatalf("Erro no setup:%v", err)
	}
	log.Printf("Download realizado com sucesso!\n")
	return []string{"teste"}, nil
}

func (c crawler) selecionaContracheque(ctx context.Context, year string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate("https://mpt.mp.br/MPTransparencia/pages/portal/remuneracaoMembrosAtivos.xhtml"),
		chromedp.Sleep(c.timeBetweenSteps),
		chromedp.SetValue(`//*[@id="j_idt136"]`, year, chromedp.BySearch),
		chromedp.Sleep(c.timeBetweenSteps),
		chromedp.Click(`//*[@id="j_idt139"]`, chromedp.BySearch, chromedp.NodeVisible),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}
func (c crawler) selecionaVerbas(ctx context.Context, year string) error {
	var buf1 []byte
	var buf2 []byte
	var buf3 []byte

	chromedp.Run(ctx,
		chromedp.DoubleClick(`//*[@id="j_idt95"]`, chromedp.BySearch, chromedp.NodeVisible),
		chromedp.Sleep(c.timeBetweenSteps),
		chromedp.FullScreenshot(&buf1, 90),
	)
	if err := ioutil.WriteFile("elementScreenshot1.png", buf1, 0o644); err != nil {
		log.Fatal(err)
	}
	chromedp.Run(ctx,
		chromedp.SetValue(`//*[@id="j_idt142"]`, year, chromedp.BySearch, chromedp.NodeReady),
		chromedp.Sleep(c.timeBetweenSteps),
		chromedp.FullScreenshot(&buf2, 90),
	)
	if err := ioutil.WriteFile("elementScreenshot2.png", buf2, 0o644); err != nil {
		log.Fatal(err)
	}
	chromedp.Run(ctx,
		chromedp.Click(`//*[@id="consultaForm"]/div[2]/div/input`, chromedp.BySearch, chromedp.NodeVisible),
		chromedp.Sleep(c.timeBetweenSteps),
		chromedp.FullScreenshot(&buf3, 90),
	)
	if err := ioutil.WriteFile("elementScreenshot3.png", buf3, 0o644); err != nil {
		log.Fatal(err)
	}
	return nil
}

// Retorna os caminhos completos dos arquivos baixados.
func (c crawler) downloadFilePath(prefix string) string {
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
	if strings.Contains(fName, "contracheques") {
		selectMonth = fmt.Sprintf(`//*[@id="tabelaRemuneracao:%d:j_idt158"]/span`, months[c.month])
	} else {
		selectMonth = fmt.Sprintf(`//*[@id="tabelaMeses:%d:linkArq"]/span`, months[c.month])
	}
	// verbas // // (ods)
	chromedp.Run(ctx,
		// Altera o diretório de download
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(c.output).
			WithEventsEnabled(true),
		// Clica no botão de download
		chromedp.Click(selectMonth, chromedp.BySearch, chromedp.NodeVisible), //*[@id="tabelaRemuneracao:8:j_idt158"]
		chromedp.Sleep(c.downloadTimeout),
	)

	if err := nomeiaDownload(c.output, fName); err != nil {
		return fmt.Errorf("erro renomeando arquivo (%s): %v", fName, err)
	}
	if _, err := os.Stat(fName); os.IsNotExist(err) {
		return fmt.Errorf("download do arquivo de %s não realizado", fName)
	}
	return nil
}
func nomeiaDownload(output, fName string) error {
	// Identifica qual foi o último arquivo
	files, err := os.ReadDir(output)
	if err != nil {
		return fmt.Errorf("erro lendo diretório %s: %v", output, err)
	}
	var newestFPath string
	var newestTime int64 = 0
	for _, f := range files {
		fPath := filepath.Join(output, f.Name())
		fi, err := os.Stat(fPath)
		if err != nil {
			return fmt.Errorf("erro obtendo informações sobre arquivo %s: %v", fPath, err)
		}
		currTime := fi.ModTime().Unix()
		if currTime > newestTime {
			newestTime = currTime
			newestFPath = fPath
		}
	}
	// Renomeia o último arquivo modificado.
	if err := os.Rename(newestFPath, fName); err != nil {
		return fmt.Errorf("erro renomeando último arquivo modificado (%s)->(%s): %v", newestFPath, fName, err)
	}
	return nil
}
