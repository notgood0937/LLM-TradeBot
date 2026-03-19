import { defineStore } from 'pinia'
import axios from 'axios'

export const useTradingStore = defineStore('trading', {
  state: () => ({
    system: { running: false, mode: 'test', is_test_mode: true, cycle_interval: 3, current_symbol: '--' },
    market: { prices: {} as Record<string, string> },
    agents: { agent_messages: [] as any[], symbol_selector: {} },
    decision: null as any,
    decision_history: [] as any[],
    logs: [] as string[],
    account: { realtime_balance: '$0.00', total_pnl: '$0.00', total_pnl_pct: '0.00%', win_rate: '0.0%', position_count: 0, total_unrealized_pnl: '$0.00', trades_count: 0, trade_history: [] as any[] },
    positions: [] as any[],
    llm_info: { provider: '--', model: '--' },
    isCleanMode: false,
    pollIntervalId: null as any,
  }),
  actions: {
    async fetchStatus() {
      try {
        const res = await axios.get('/api/status')
        const data = res.data
        this.system = data.system || this.system
        this.market = data.market || this.market
        this.agents = data.agents || this.agents
        this.decision = data.decision || this.decision
        this.decision_history = data.decision_history || this.decision_history
        this.logs = data.logs || this.logs

        let act = data.account
        if ((!act || Object.keys(act).length === 0) && data.virtual_account) {
          const va = data.virtual_account
          const unrealized = va.total_unrealized_pnl || 0
          const initialBalance = va.initial_balance || 0
          const totalEquity = va.current_balance + unrealized
          act = {
            realtime_balance: totalEquity.toLocaleString('en-US', {style:'currency', currency:'USD'}),
            total_pnl: (totalEquity - initialBalance).toLocaleString('en-US', {style:'currency', currency:'USD', signDisplay: 'always'}),
            total_unrealized_pnl: unrealized.toLocaleString('en-US', {style:'currency', currency:'USD', signDisplay: 'always'})
          }
        } else if (act) {
          const te = act.total_equity ?? act.totalMarginBalance ?? 0
          const pnl = act.total_pnl ?? act.realized_pnl ?? 0
          const unpnl = act.total_unrealized_pnl ?? act.unrealized_pnl ?? act.totalUnrealizedProfit ?? 0
          act.realtime_balance = te.toLocaleString('en-US', {style:'currency', currency:'USD'})
          act.total_pnl = pnl.toLocaleString('en-US', {style:'currency', currency:'USD', signDisplay: 'always'})
          act.total_unrealized_pnl = unpnl.toLocaleString('en-US', {style:'currency', currency:'USD', signDisplay: 'always'})
        }

        this.account = { ...this.account, ...act }
        if (data.trade_history) this.account.trade_history = data.trade_history
        this.positions = data.positions || this.positions
      } catch (error) {
        console.error('Failed to fetch status', error)
      }
    },
    async fetchInitialConfig() {
      try {
        const res = await axios.get('/api/agents/settings')
        if (res.data?.llm_info) {
          this.llm_info.provider = res.data.llm_info.provider || '--'
          this.llm_info.model = res.data.llm_info.model || '--'
        }
      } catch (error) {
        console.error('Failed to fetch LLM config', error)
      }
    },
    startPolling() {
      this.fetchInitialConfig()
      this.fetchStatus()
      if (!this.pollIntervalId) {
        this.pollIntervalId = setInterval(this.fetchStatus, 2000)
      }
    },
    stopPolling() {
      if (this.pollIntervalId) {
        clearInterval(this.pollIntervalId)
        this.pollIntervalId = null
      }
    },
    toggleCleanMode() {
      this.isCleanMode = !this.isCleanMode
    },
    async triggerControl(action: string) {
      try {
        await axios.post('/api/control', { action, mode: this.system.mode, interval: this.system.cycle_interval })
        this.fetchStatus()
      } catch (err) {
        console.error('Control failed:', err)
      }
    }
  }
})
