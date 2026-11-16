# SEP Cloud Run 版：交接規格

> 這份文件是寫給「接手帶使用者做完這個專案的 AI 模型」看的。
> 每次開新對話，先讀完這一份，再只讀「目前步驟」對應的那一份規格，不要一次全讀。
> 最後更新：2026-11-16

## 1. 專案在做什麼

SEP（Serverless Execution Platform）原本跑在 K8s：Engine 收到 `POST /api/run` 就建一個 K8s Job 跑批次任務，Cleaner 常駐收割異常 Pod。

分支 `dev-GcpCloudRun` 要做一個 **GCP Cloud Run 版**，目的是不用養 K8s 叢集，並且**完全落在免費額度內**。K8s 版要保持可以正常編譯和運作。

| K8s 版 | Cloud Run 版 |
|---|---|
| Engine Deployment + Service | Cloud Run Service `sep-engine`（不公開、min=0） |
| Cleaner Deployment（常駐輪詢） | Cloud Run Job `sep-cleaner` + Cloud Scheduler 定時觸發，跑一次就結束 |
| ConfigMap `sep-system-quotas` | Terraform 變數 → 環境變數 `SYSTEM_QUOTAS_JSON` |
| `BatchV1().Jobs().Create()` | Cloud Run Admin API v2：`CreateJob` 之後 `RunJob` |
| `ttlSecondsAfterFinished: 300` | Cloud Run Job 沒有 TTL，由 Cleaner 刪除跑完的 job |
| ServiceAccount + RBAC | GCP Service Account + IAM |
| Helm values | `terraform.tfvars` |

## 2. 怎麼帶這位使用者（最重要）

使用者的目的是**學會**，不是只要結果。違反下面任何一條，使用者都會喊停。

1. **Terraform 檔案由使用者自己打。** 你在對話裡貼出範例（從 `specs/_reference/terraform/` 讀），逐段解釋語法，使用者抄寫進 `terraform/`。不要用工具直接寫 `terraform/` 底下的 `.tf` 檔。
2. **雲端指令由使用者自己跑。** `gcloud`、`terraform apply`、`terraform destroy`、部署腳本都給指令讓使用者執行，並說明預期輸出。唯讀的檢查（讀檔、`terraform fmt -check`、`terraform validate`、`git status`）你可以自己跑。
3. **一次只做一小步。** 一個步驟一個概念，做完等使用者貼輸出，確認後才進下一步。使用者說過「有點太快了」。
4. **每一步都要講「為什麼」。** 這一步在做什麼、為什麼需要、不做會怎樣。指令的每個參數、HCL 的每一行都要解釋。
5. **使用者對 Terraform 語法很不熟。** 第一次出現的語法（`resource`、引用、`locals`、字串插值、`depends_on`、`jsonencode`、`output` 等）要當成新東西教。已學過的見第 5 節。
6. **使用者常問延伸問題**（「外面公司怎麼做」「這會不會被盜」「這要不要錢」）。直接、誠實回答，然後回到主線。
7. **錯誤先解釋再給解法。** 使用者貼錯誤訊息時，指出是訊息裡的哪一句、原因是什麼，再給修正。
8. **Go 程式和腳本由誰寫還沒問過使用者。** 開始 G 階段前先問：要自己打，還是由你套用後講解。原計畫是由 AI 寫。
9. 全程使用繁體中文。使用者的訊息偶爾有注音打錯的亂碼，忽略即可。
10. 使用者開新 terminal 後 `export` 的變數會消失，給指令時盡量寫完整的值，或提醒先重新 export。

## 3. 不可更動的決策

| 項目 | 決策 | 原因 |
|---|---|---|
| CI/CD | 使用者在本機手動執行 `scripts/deploy_cloudrun.sh` | repo 是公開的，GitHub 和 GCP 之間不建立任何連線 |
| 憑證 | 只有本機的 `gcloud auth login` 和 ADC | **不要**建議 SA key、Workload Identity Federation、Cloud Build trigger、GitHub Actions 部署到 GCP |
| Engine 對外 | 不公開，**不可**給 `allUsers` 或 `allAuthenticatedUsers` 任何權限 | 自己用時透過 `gcloud run services proxy` |
| Image 建置 | `gcloud builds submit`（Cloud Build） | Mac 是 arm64，Cloud Run 要 amd64 |
| 基礎設施 | Terraform，state 放 GCS | 可以 `plan` 看差異、`destroy` 清乾淨 |
| 任務 job | Engine 執行期用 API 建立，不進 Terraform | Terraform 只管固定的基礎設施 |
| 費用 | 全部落在免費額度 | 任何會產生費用的建議都要先講明 |
| 進 repo 的檔案 | 不可含 Project ID、billing account ID | 這些值只放在 `terraform.tfvars`（已 gitignore）。這份規格也用 `<PROJECT_ID>` 代替 |

## 4. 環境現況

需要實際值時用指令查，不要寫進任何會 commit 的檔案：

```bash
gcloud config get-value project                 # PROJECT_ID
gcloud billing projects describe <PROJECT_ID>   # billing account
```

| 項目 | 值 |
|---|---|
| 工具 | gcloud 592.0.0、Terraform 1.17.1、Go 1.26（本機） |
| Region | `asia-east1`（Cloud Run、Artifact Registry、Scheduler） |
| 已開啟的 API | run、cloudbuild、artifactregistry、cloudscheduler、iam、cloudresourcemanager、billingbudgets |
| State bucket | `gs://<PROJECT_ID>-tfstate`，位置 `us-central1`（為了免費額度），versioning 已開 |
| Terraform backend | `gcs`，`prefix = "sep"`，bucket 用 `-backend-config` 傳入 |
| ADC | 已完成 `gcloud auth application-default login`，quota project 已設 |
| Budget | `sep-budget`，150 TWD，20% / 50% / 100% 寄信。只通知，不會停止計費 |
| google provider | `7.46.2`（鎖在 `terraform/.terraform.lock.hcl`） |
| 使用者帳號權限 | 專案 owner。已確認 owner 含 `run.routes.invoke` 和 `iam.serviceAccounts.actAs`，所以不需要另外給自己 invoker 權限 |

`terraform init` 的完整指令（只有換 provider 或 backend 時才需要重跑）：

```bash
cd terraform && terraform init -backend-config="bucket=<PROJECT_ID>-tfstate"
```

## 5. 使用者已經學會的

- `init` = 下載 provider + 連上 backend，類似 `npm install`；`.terraform/` 不進版控，`.terraform.lock.hcl` 要進版控。
- 區塊（沒有 `=`）和引數（有 `=`）的差別；區塊的標籤。
- `required_version`（主程式）和 provider `version` 的差別；`~>` 的意思。
- backend 是什麼、為什麼 `backend` 區塊裡不能用變數。
- 檔名對 Terraform 沒有意義，同資料夾的 `.tf` 會合併。
- bucket（儲存空間）和 budget（預算告警）的差別；ADC 和 `gcloud auth login` 的差別。

**已講解但還沒實作**：`variable` 的宣告、`terraform.tfvars` 給值、`var.x` 引用、`provider` 區塊、`fmt` / `validate` / `plan`。T1 可以快速複習，不用從頭講。

**還沒接觸**：`resource`、資源之間的引用、`locals`、字串插值、`plan` 輸出的 `+ / ~ / -` 符號、`apply`、`output`、`depends_on`、`jsonencode` / `join` / `concat`、`destroy`。

## 6. 進度

圖例：✅ 完成　⬜ 未開始　👤 使用者操作　🤖 AI 可以直接做

| 步驟 | 內容 | 狀態 | 規格 |
|---|---|---|---|
| P0 | 安裝 gcloud、terraform | ✅ | |
| P1 | 開 API、state bucket、ADC、budget | ✅ | |
| T0 | `terraform/main.tf`（版本宣告）、`backend.tf`、`terraform init` | ✅ 已 commit `4792704` | |
| **T1** | `variables.tf`、`terraform.tfvars`、`provider` 區塊，第一次 `plan` | ⬜ **← 從這裡開始** | [01](01-terraform-foundation.md) |
| T2 | `artifact_registry.tf`，第一次 `apply` | ⬜ | [01](01-terraform-foundation.md) |
| T3 | `build.tf`：builder SA、原始碼 bucket、權限 | ⬜ | [01](01-terraform-foundation.md) |
| T4 | `iam.tf`：四個執行期 SA 和權限 | ⬜ | [01](01-terraform-foundation.md) |
| G1 | Go：設定、Launcher 介面、K8s launcher 改名 | ⬜ | [02](02-go.md) |
| G2 | Go：`CloudRunLauncher` | ⬜ | [02](02-go.md) |
| G3 | Go：`CloudRunCleaner` | ⬜ | [02](02-go.md) |
| G4 | Dockerfile 升 Go 版本、`go mod tidy`、確認 K8s 版能編譯 | ⬜ | [02](02-go.md) |
| B1 | `cloudbuild.yaml`、`.gcloudignore`、`.gitignore` 補 `.DS_Store` | ⬜ | [03](03-build-and-deploy.md) |
| B2 | `scripts/deploy_cloudrun.sh`，第一次 build（`--build-only`） | ⬜ | [03](03-build-and-deploy.md) |
| T5 | `engine.tf`，Engine 上線 | ⬜ | [03](03-build-and-deploy.md) |
| T6 | `cleaner.tf`：Cleaner job 和 Scheduler | ⬜ | [03](03-build-and-deploy.md) |
| T7 | `outputs.tf`、`terraform.tfvars.example` | ⬜ | [03](03-build-and-deploy.md) |
| V1 | 驗證：不公開、proxy 呼叫、任務執行、失敗情境、Cleaner | ⬜ | [04](04-verify-and-wrapup.md) |
| V2 | 調低 Cloud Run 配額、費用檢查 | ⬜ | [04](04-verify-and-wrapup.md) |
| V3 | `destroy` 演練 | ⬜ | [04](04-verify-and-wrapup.md) |
| D1 | README、架構圖更新 | ⬜ | [04](04-verify-and-wrapup.md) |

**每完成一個步驟，更新這張表的狀態和「← 從這裡開始」的位置。** 這是下一次對話接續的依據。

### 為什麼是這個順序

Cloud Run 服務建立時 image 必須已經存在，而 image 要先有 Artifact Registry 才能 push。所以順序是：先用 Terraform 建不需要 image 的東西（T1–T4），寫好程式並 build 出第一個 image（G、B），再用 Terraform 建 Engine 和 Cleaner（T5–T6）。這樣每一步都可以直接 `apply`，不需要 `-target`。

## 7. 參考實作（已移除）

原本放在 `specs/_reference/` 的參考程式碼，已在 2026-11-17 合併進 repo 本體後刪除，現在直接看實際檔案：

| 內容 | 位置 |
|---|---|
| Terraform | `terraform/*.tf`。`engine.tf.pending`、`cleaner.tf.pending`、`outputs.tf.pending` 要等第一個 image build 完才把 `.pending` 拿掉 |
| Go | `common/config`、`common/model/labels.go`、`services/engine/service/{launcher,k8s_launcher,cloudrun_launcher}.go`、`services/cleaner/service/cloudrun_cleaner.go` 等 |
| Build | 根目錄的 `cloudbuild.yaml`、`.gcloudignore` |

驗證程度：`go build`、`go vet`、`terraform validate` 通過；`plan` 顯示 13 個新增資源。**尚未 apply，尚未對真的 Cloud Run 執行。**
使用者不喜歡 shell 腳本，`deploy_cloudrun.sh` 沒有合併，部署流程待與使用者決定（見對話）。

## 8. 其他檔案

- `待辦事項.md`：最初的計畫，僅供對照。細節已被這份規格取代（順序、IAM 設計、Cleaner 排程都調整過），兩者衝突時以這份規格為準。
- `charts/`、`scripts/deploy.sh`、`.github/workflows/CI-Build.yaml`：K8s 版的東西，不要動。
- `services/*/.env*` 和 `services/engine/engine`：早已被 git 追蹤，內容只有非機密設定。`.gitignore` 對已追蹤的檔案無效，這是既有狀況，不在這次範圍內，除非使用者要處理。
