import { stageAllowsAction } from "./utils";
import type { OnboardingFlowController } from "./types";

export function ConnectStagePanel({
  controller,
}: {
  controller: OnboardingFlowController;
}) {
  const {
    connectOnly,
    connectOnlyDescription,
    connectOnlyTitle,
    connectMode,
    connectionStage,
    activeApp,
    onboardingSession,
    connectError,
    actionBusy,
    isReadOnlyApp,
    manualForm,
    mode,
    changeConnectMode,
    resetQRCodeSession,
    retryQRCodeVerification,
    setManualForm,
    submitManualConnect,
  } = controller;

  const description = connectOnly
    ? connectOnlyDescription
    : connectionStage?.summary || "接入并验证一个可用的飞书应用。";
  const canStartQRCode = connectOnly
    ? true
    : stageAllowsAction(connectionStage, "start_qr");
  const canSubmitManual = connectOnly
    ? true
    : stageAllowsAction(connectionStage, "submit_manual");

  return (
    <section className="step-section">
      <div className="step-stage-head">
        <h2>{connectOnly ? connectOnlyTitle : "飞书连接"}</h2>
        <p>{description}</p>
      </div>
      {!connectOnly && activeApp ? (
        <div className="notice-banner good">
          当前应用：{activeApp.name || activeApp.id}
          {activeApp.readOnly ? "（运行时接管，只能验证，不可在网页修改）" : ""}
        </div>
      ) : null}
      <div className="choice-toggle">
        <button
          className={connectMode === "qr" ? "primary-button" : "ghost-button"}
          type="button"
          onClick={() => changeConnectMode("qr")}
        >
          扫码创建
        </button>
        <button
          className={connectMode === "manual" ? "primary-button" : "ghost-button"}
          type="button"
          onClick={() => changeConnectMode("manual")}
        >
          手动输入
        </button>
      </div>
      {connectMode === "qr" ? (
        <div className="panel">
          <div className="scan-preview">
            <div>
              <h4 style={{ margin: 0 }}>扫码创建</h4>
              <p className="support-copy">
                请使用飞书扫码完成创建，页面会按当前流程自动刷新连接阶段。
              </p>
              <div className="scan-frame">
                {onboardingSession?.qrCodeDataUrl ? (
                  <img alt="飞书扫码创建二维码" src={onboardingSession.qrCodeDataUrl} />
                ) : (
                  <span>{canStartQRCode ? "二维码准备中" : "当前暂时不能发起扫码"}</span>
                )}
              </div>
            </div>
            <div className="detail-stack">
              {onboardingSession?.status === "pending" ? (
                <div className="notice-banner warn">正在等待扫码结果...</div>
              ) : null}
              {onboardingSession?.status === "ready" && !connectError ? (
                <div className="notice-banner good">扫码成功，正在刷新当前连接状态...</div>
              ) : null}
              {onboardingSession?.status === "failed" ||
              onboardingSession?.status === "expired" ||
              connectError ? (
                <div className="notice-banner danger">
                  {connectError || "当前扫码没有继续成功，请重新开始。"}
                </div>
              ) : null}
              <div className="button-row">
                {(connectError ||
                  onboardingSession?.status === "failed" ||
                  onboardingSession?.status === "expired") && (
                  <button
                    className="secondary-button"
                    type="button"
                    disabled={actionBusy === "qr-start" || !canStartQRCode}
                    onClick={resetQRCodeSession}
                  >
                    重新扫码
                  </button>
                )}
                {onboardingSession?.status === "ready" && connectError ? (
                  <button
                    className="secondary-button"
                    type="button"
                    disabled={actionBusy === "qr-complete"}
                    onClick={retryQRCodeVerification}
                  >
                    重新验证
                  </button>
                ) : null}
                <button
                  className="ghost-button"
                  type="button"
                  onClick={() => changeConnectMode("manual")}
                >
                  改用手动输入
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="panel">
          {isReadOnlyApp ? (
            <div className="notice-banner warn">
              当前机器人信息由当前运行环境提供，网页里不能修改，只能完成连接验证。
            </div>
          ) : null}
          <div className="form-grid">
            <label className="field">
              <span>
                App ID <em className="field-required">*</em>
              </span>
              <input
                aria-label="App ID"
                disabled={isReadOnlyApp}
                placeholder="请输入 App ID"
                value={manualForm.appId}
                onChange={(event) =>
                  setManualForm((current) => ({
                    ...current,
                    appId: event.target.value,
                  }))
                }
              />
            </label>
            <label className="field">
              <span>
                App Secret <em className="field-required">*</em>
              </span>
              <input
                aria-label="App Secret"
                disabled={isReadOnlyApp}
                placeholder="请输入 App Secret"
                value={manualForm.appSecret}
                onChange={(event) =>
                  setManualForm((current) => ({
                    ...current,
                    appSecret: event.target.value,
                  }))
                }
              />
            </label>
            <label className="field form-grid-span-2">
              <span>机器人名称（可选）</span>
              <input
                aria-label="机器人名称（可选）"
                disabled={isReadOnlyApp}
                placeholder={mode === "setup" ? "例如：团队机器人" : "例如：运营机器人"}
                value={manualForm.name}
                onChange={(event) =>
                  setManualForm((current) => ({
                    ...current,
                    name: event.target.value,
                  }))
                }
              />
            </label>
          </div>
          <div className="button-row">
            <button
              className="primary-button"
              type="button"
              disabled={actionBusy === "manual-connect" || !canSubmitManual}
              onClick={() => void submitManualConnect()}
            >
              {mode === "setup" ? "验证并继续" : "验证并保存"}
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
