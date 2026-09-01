# ============================================================
# sep/flow.py
# 在 K8s Pod 內直接執行的 Prefect Flow
# 由 SEP (Serverless Execution Platform) Engine 建立的 K8s Job 來呼叫
# ============================================================

from prefect import task, flow, get_run_logger
import time
import os


@task(name="批次資料處理")
def batch_process():
    logger = get_run_logger()
    logger.info("⚡ SEP Task Job 啟動！")
    logger.info(f"🐳 Pod hostname : {os.environ.get('HOSTNAME', 'unknown')}")
    logger.info(f"📡 Prefect API  : {os.environ.get('PREFECT_API_URL', 'not set')}")
    time.sleep(2)
    logger.info("✅ 批次完成，Pod 即將銷毀")
    return "done"


@flow(name="SEP Batch Flow")
def sep_flow():
    """
    由 SEP Engine → K8s Job → Pod 啟動執行此 Flow。
    無需常駐 Worker，執行完 Pod 自動銷毀。
    """
    result = batch_process()
    return result


if __name__ == "__main__":
    sep_flow()
