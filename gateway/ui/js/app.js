function app() {
 return {
 loaded: false,
 route: '/',
 store: null,

 toast: { show: false, msg: '', type: 'success' },
 confirm: { show: false, title: '', msg: '', action: null },

 createInstance: { open: false, sdkNo: '', port: '' },
 deviceForm: { open: false, editing: false, id: 0, name: '', sn: '', activation: '', password: '0', ip: '', ethernet_port: '5005', sdk_no: 0 },
  deviceDetail: {},
  opResult: '',
 syncInterval: 60,
 scanlogPage: { sn: '', data: [], total: 0, page: 1, size: 50, status: {}, compare: {scanlog: {}, users: {}}, syncing: false, comparing: false, sdkNo: '', _pollTimer: null },
  usersPage: { sn: '', data: [], total: 0, page: 1, size: 50, status: {}, compare: {scanlog: {}, users: {}}, syncing: false, sdkNo: '', comparing: false, _pollTimer: null, syncLimit: 30 },
 testPage: { sn: '', sdkNo: '', result: '', loading: false },

 isDeviceDetail() {
 return this.route.startsWith('/devices/') && this.route !== '/devices';
 },

init() {
        if (this._initialized) return;
        this._initialized = true;
        const self = this;
 const store = Alpine.reactive({
 instances: [],
 devices: [],
 jobs: [],
 logs: [],
 health: { total: 0, running: 0, stopped: 0, error: 0, instances: [] },
 _pending: {},
 _scanlogDeviceCount: null,
 _usersDeviceCount: null,
 async api(path, method, body) {
 const key = method + ':' + path;
 if (this._pending[key]) return this._pending[key];
 const promise = (async () => {
 try {
 const opts = { method, headers: { 'Content-Type': 'application/json' } };
 if (body) opts.body = JSON.stringify(body);
 const r = await fetch('/api' + path, opts);
 const data = await r.json();
 if (!r.ok) throw new Error(data.error || r.statusText);
 return data;
 } catch (e) {
 self.toast = { show: true, msg: e.message, type: 'error' };
 setTimeout(() => { self.toast.show = false }, 3000);
 throw e;
 }
 })();
 this._pending[key] = promise;
 try {
 return await promise;
 } finally {
 delete this._pending[key];
 }
 },

 async fetchInstances() { this.instances = await this.api('/instances', 'GET') || []; },
 async fetchDevices() { this.devices = await this.api('/devices', 'GET') || []; },
 async fetchJobs() { this.jobs = await this.api('/jobs', 'GET') || []; },
 async fetchHealth() {
 try {
 this.health = await this.api('/sync/status', 'GET');
 } catch (e) {
 }
 },

 startLogStream() {
 if (this._es) { this._es.close(); delete this._es; }
 const self = this;
 fetch('/api/logs').then(r => r.json()).then(d => { self.logs = d || []; }).catch(() => {});
 const es = new EventSource('/api/logs/stream');
 es.onmessage = (e) => {
 try {
 const log = JSON.parse(e.data);
 if (Array.isArray(log)) { self.logs = log; }
 else { self.logs.push(log); if (self.logs.length > 500) self.logs.shift(); }
 const el = document.querySelector('[x-ref="logContainer"]');
 if (el) { setTimeout(() => { el.scrollTop = el.scrollHeight; }, 50); }
 } catch (ex) {}
 };
 es.onerror = () => {};
 },

 async syncReload() {
 await this.api('/sync/reload', 'POST', {});
 self.toast = { show: true, msg: 'Sync reloaded from Device.ini', type: 'success' };
 setTimeout(() => { self.toast.show = false }, 3000);
 await this.fetchDevices();
 await this.fetchInstances();
 },

 async createInstance(sdkNo, port) {
 await this.api('/instances', 'POST', { sdk_no: sdkNo, port: port });
 self.toast = { show: true, msg: 'Instance created', type: 'success' };
 setTimeout(() => { self.toast.show = false }, 3000);
 await this.fetchInstances();
 },
 async startInstance(sdkNo) {
 await this.api('/instances/' + sdkNo + '/start', 'POST');
 self.toast = { show: true, msg: 'Instance started', type: 'success' };
 setTimeout(() => { self.toast.show = false }, 3000);
 await this.fetchInstances();
 },
 async stopInstance(sdkNo) {
 await this.api('/instances/' + sdkNo + '/stop', 'POST');
 self.toast = { show: true, msg: 'Instance stopped', type: 'success' };
 setTimeout(() => { self.toast.show = false }, 3000);
 await this.fetchInstances();
 },
 async restartInstance(sdkNo) {
 await this.api('/instances/' + sdkNo + '/restart', 'POST');
 self.toast = { show: true, msg: 'Instance restarted', type: 'success' };
 setTimeout(() => { self.toast.show = false }, 3000);
 await this.fetchInstances();
 },
 async deleteInstance(sdkNo) {
 await this.api('/instances/' + sdkNo, 'DELETE');
 self.toast = { show: true, msg: 'Instance deleted', type: 'success' };
 setTimeout(() => { self.toast.show = false }, 3000);
 await this.fetchInstances();
 },

 async createDevice(data) { await this.api('/devices', 'POST', data); await this.fetchDevices(); },
 async updateDevice(id, data) { await this.api('/devices/' + id, 'PUT', data); await this.fetchDevices(); },
 async deleteDevice(id) { await this.api('/devices/' + id, 'DELETE'); await this.fetchDevices(); },
 async getDeviceBySN(sn) {
 const devices = await this.api('/devices', 'GET') || [];
 const d = devices.find(d => d.sn === sn || d.sn === decodeURIComponent(sn));
 if (!d) throw new Error('Device not found: ' + sn);
 self.deviceDetail = d;
 return d;
 },

 async deviceOp(sn, action) {
 const actMap = {
 'dev/info': { path: '/' + sn + '/info', method: 'GET' },
 'dev/settime': { path: '/' + sn + '/time', method: 'POST' },
 'dev/init': { path: '/' + sn + '/init', method: 'POST' },
 'dev/deladmin': { path: '/' + sn + '/deladmin', method: 'POST' },
 'scanlog/new': { path: '/' + sn + '/scan/new', method: 'GET' },
 'scanlog/all': { path: '/' + sn + '/scan/all', method: 'GET' },
 'scanlog/del': { path: '/' + sn + '/scan/delete', method: 'POST' },
 'user/all': { path: '/' + sn + '/users', method: 'GET' },
 'user/set': { path: '/' + sn + '/users', method: 'POST' },
 'log/del': { path: '/' + sn + '/log/del', method: 'POST' },
 };
 const m = actMap[action];
 if (!m) return;
 try {
 const opts = { method: m.method, headers: { 'Content-Type': 'application/json' } };
 const r = await fetch('/api/devices' + m.path, opts);
 const data = await r.json();
 self.opResult = JSON.stringify(data, null, 2);
 } catch (e) {
 self.opResult = 'Error: ' + e.message;
 }
 },

 async fetchScanlogPage() {
 const sn = self.scanlogPage.sn;
 if (!sn) return;
 try {
 const r = await fetch('/api/devices/' + encodeURIComponent(sn) + '/scan/logs?page=' + self.scanlogPage.page + '&size=' + self.scanlogPage.size);
 const d = await r.json();
 self.scanlogPage.data = d.data || [];
 self.scanlogPage.total = d.total || 0;
 } catch (e) { self.scanlogPage.data = []; self.scanlogPage.total = 0; }
 },
 async fetchScanlogStatus(sdkNo) {
 const sn = self.scanlogPage.sn;
 if (!sn) { self.scanlogPage.status = {}; self.scanlogPage.compare = {}; return; }
 try {
 const r = await fetch('/api/devices/' + encodeURIComponent(sn) + '/absen/info');
 self.scanlogPage.status = await r.json();
 } catch (e) { self.scanlogPage.status = {}; }
 if (sdkNo !== undefined) {
 try {
 const cached = self.store._scanlogDeviceCount;
 if (cached !== null) {
 const cr = await fetch('/api/devices/' + encodeURIComponent(sn) + '/absen/compare?sdk_no=' + sdkNo + '&skip_device=1');
 const data = await cr.json();
 data.scanlog.device = cached;
 data.scanlog.synced = data.scanlog.local === cached && cached > 0;
 self.scanlogPage.compare = data;
 } else {
 const cr = await fetch('/api/devices/' + encodeURIComponent(sn) + '/absen/compare?sdk_no=' + sdkNo);
 const data = await cr.json();
 self.scanlogPage.compare = data;
 if (data.scanlog && data.scanlog.device > 0) {
 self.store._scanlogDeviceCount = data.scanlog.device;
 }
 }
 } catch (e) { self.scanlogPage.compare = {}; }
 } else {
 self.scanlogPage.compare = {};
 }
 },
 async onScanlogDeviceChange() {
 self.scanlogPage.page = 1;
 self.scanlogPage.syncing = false;
 self.store._scanlogDeviceCount = null;
 this.stopProgressPoll();
 await this.fetchScanlogPage();
 await this.fetchScanlogStatus();
 },

async doSyncScanlog() {
 const sn = self.scanlogPage.sn;
 if (!sn) return;
 self.scanlogPage.syncing = true;
 this.startProgressPoll();
 try {
 const body = { sdk_no: parseInt(self.scanlogPage.sdkNo) || 0 };
 const dc = self.store._scanlogDeviceCount;
 if (dc != null && dc > 0) body.device_scanlog = dc;
 const r = await fetch('/api/devices/' + encodeURIComponent(sn) + '/scan/sync', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
 const data = await r.json();
 if (r.ok) {
 const msg = data.inserted !== undefined ? ('Sync complete: ' + data.count + ' records (' + data.inserted + ' new)') : ('Sync complete: ' + (data.count || 'OK'));
 self.toast = { show: true, msg: msg, type: 'success' };
 setTimeout(() => { self.toast.show = false }, 3000);
 }
 } catch (e) {
 self.toast = { show: true, msg: 'Sync failed: ' + (e.message || e), type: 'error' };
 setTimeout(() => { self.toast.show = false }, 3000);
 }
 self.scanlogPage.syncing = false;
 this.stopProgressPoll();
 self.store._scanlogDeviceCount = null;
 await this.fetchScanlogPage();
 await this.fetchScanlogStatus(parseInt(self.scanlogPage.sdkNo) || 0);
 },

 async doCompare() {
 const sn = self.scanlogPage.sn;
 if (!sn) return;
 self.scanlogPage.comparing = true;
 self.store._scanlogDeviceCount = null;
 try {
        await this.fetchScanlogStatus(parseInt(self.scanlogPage.sdkNo) || 0);
    } catch (e) {
        self.toast = { show: true, msg: 'Compare failed: ' + (e.message || e), type: 'error' };
        setTimeout(() => { self.toast.show = false }, 3000);
    }
    self.scanlogPage.comparing = false;
  },

  startProgressPoll() {
    this.stopProgressPoll();
    self.scanlogPage._pollTimer = setInterval(async () => {
        if (!self.scanlogPage.syncing) { this.stopProgressPoll(); return; }
        await this.fetchScanlogStatus(parseInt(self.scanlogPage.sdkNo) || 0);
        await this.fetchScanlogPage();
    }, 2000);
  },
  stopProgressPoll() {
    if (self.scanlogPage._pollTimer) {
        clearInterval(self.scanlogPage._pollTimer);
        self.scanlogPage._pollTimer = null;
    }
  },

 async fetchUsersPage() {
 const sn = self.usersPage.sn;
 if (!sn) return;
 try {
 const r = await fetch('/api/devices/' + encodeURIComponent(sn) + '/absen/users?page=' + self.usersPage.page + '&size=' + self.usersPage.size);
 const d = await r.json();
 self.usersPage.data = d.data || [];
 self.usersPage.total = d.total || 0;
 } catch (e) { self.usersPage.data = []; self.usersPage.total = 0; }
 },
 async fetchUsersStatus(sdkNo) {
 const sn = self.usersPage.sn;
 if (!sn) { self.usersPage.status = {}; self.usersPage.compare = {}; return; }
 try {
 const r = await fetch('/api/devices/' + encodeURIComponent(sn) + '/absen/info');
 self.usersPage.status = await r.json();
 } catch (e) { self.usersPage.status = {}; }
 if (sdkNo !== undefined) {
 try {
 const cached = self.store._usersDeviceCount;
 if (cached !== null) {
 const cr = await fetch('/api/devices/' + encodeURIComponent(sn) + '/absen/compare?sdk_no=' + sdkNo + '&skip_device=1');
 const data = await cr.json();
 data.users.device = cached;
 data.users.synced = data.users.local === cached && cached > 0;
 self.usersPage.compare = data;
 } else {
 const cr = await fetch('/api/devices/' + encodeURIComponent(sn) + '/absen/compare?sdk_no=' + sdkNo);
 const data = await cr.json();
 self.usersPage.compare = data;
 if (data.users && data.users.device > 0) {
 self.store._usersDeviceCount = data.users.device;
 }
 }
 } catch (e) { self.usersPage.compare = {}; }
 } else {
 self.usersPage.compare = {};
 }
 },
 async onUsersDeviceChange() {
 self.usersPage.page = 1;
 self.usersPage.syncing = false;
 self.store._usersDeviceCount = null;
 this.stopUsersProgressPoll();
 try {
 const r = await fetch('/api/config');
 const data = await r.json();
 const cfg = data.find(c => c.key === 'user_sync_limit');
 if (cfg) self.usersPage.syncLimit = parseInt(cfg.value) || 30;
 } catch (e) {}
 await this.fetchUsersPage();
 await this.fetchUsersStatus();
 },
 async doSyncUsers() {
 const sn = self.usersPage.sn;
 if (!sn) return;
 self.usersPage.syncing = true;
 this.startUsersProgressPoll();
 try {
 const r = await fetch('/api/devices/' + encodeURIComponent(sn) + '/users/sync', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ sdk_no: parseInt(self.usersPage.sdkNo) || 0, limit: self.usersPage.syncLimit }) });
 const data = await r.json();
 if (r.ok) {
 const msg = data.user_count !== undefined ? ('Sync complete: ' + data.user_count + ' users') : ('Sync complete: ' + (data.status || 'OK'));
 self.toast = { show: true, msg: msg, type: 'success' };
 setTimeout(() => { self.toast.show = false }, 3000);
 }
 } catch (e) {
 self.toast = { show: true, msg: 'Sync failed: ' + (e.message || e), type: 'error' };
 setTimeout(() => { self.toast.show = false }, 3000);
 }
 self.usersPage.syncing = false;
 this.stopUsersProgressPoll();
 self.store._usersDeviceCount = null;
 await this.fetchUsersPage();
 await this.fetchUsersStatus(parseInt(self.usersPage.sdkNo) || 0);
 },

 async doCompareUsers() {
 const sn = self.usersPage.sn;
 if (!sn) return;
 self.usersPage.comparing = true;
 self.store._usersDeviceCount = null;
 try {
 await this.fetchUsersStatus(parseInt(self.usersPage.sdkNo) || 0);
 } catch (e) {
 self.toast = { show: true, msg: 'Compare failed: ' + (e.message || e), type: 'error' };
 setTimeout(() => { self.toast.show = false }, 3000);
 }
 self.usersPage.comparing = false;
 },

 startUsersProgressPoll() {
 this.stopUsersProgressPoll();
 self.usersPage._pollTimer = setInterval(async () => {
 if (!self.usersPage.syncing) { this.stopUsersProgressPoll(); return; }
 await this.fetchUsersStatus(parseInt(self.usersPage.sdkNo) || 0);
 await this.fetchUsersPage();
 }, 2000);
 },
 stopUsersProgressPoll() {
 if (self.usersPage._pollTimer) {
 clearInterval(self.usersPage._pollTimer);
 self.usersPage._pollTimer = null;
 }
 },

 async runDeviceInfo() {
 self.testPage.result = '';
 self.testPage.loading = true;
 try {
  const r = await fetch('/api/test/device-info', {
   method: 'POST',
   headers: { 'Content-Type': 'application/json' },
   body: JSON.stringify({ sn: self.testPage.sn, sdk_no: parseInt(self.testPage.sdkNo) || 0 }),
  });
  const data = await r.json();
  if (r.ok) {
   self.testPage.result = JSON.stringify(data, null, 2);
  } else {
   self.testPage.result = 'Error: ' + (data.error || r.statusText);
  }
 } catch (e) {
  self.testPage.result = 'Error: ' + e.message;
 }
 self.testPage.loading = false;
 },

 async fetchUserTemplates(sn, pin) {
 try {
 const r = await fetch('/api/devices/' + encodeURIComponent(sn) + '/users/' + encodeURIComponent(pin) + '/templates');
 return await r.json();
 } catch (e) { return []; }
 },
 });

 this.store = store;

 this.route = window.location.pathname;
 this.loaded = false;

 store.fetchHealth();

 if (this.route === '/') {
 } else if (this.route === '/instances') {
 store.fetchInstances();
 } else if (this.route === '/devices') {
 store.fetchDevices();
 } else if (this.route.startsWith('/devices/') && this.route !== '/devices') {
 const sn = this.route.split('/')[2];
 if (sn) store.getDeviceBySN(sn);
 } else if (this.route === '/jobs') {
 store.fetchJobs();
 } else if (this.route === '/scanlog') {
 store.fetchDevices();
 store.fetchInstances();
 const params = new URLSearchParams(window.location.search);
 const snFromQuery = params.get('sn');
 if (snFromQuery) {
 this.scanlogPage.sn = snFromQuery;
 this.store.onScanlogDeviceChange();
 }
 } else if (this.route === '/users') {
 store.fetchDevices();
 store.fetchInstances();
 const params = new URLSearchParams(window.location.search);
 const snFromQuery = params.get('sn');
 if (snFromQuery) {
 this.usersPage.sn = snFromQuery;
 this.store.onUsersDeviceChange();
 } else {
 this.loadUserSyncLimit();
 }
 } else if (this.route === '/test') {
 store.fetchDevices();
 store.fetchInstances();
 } else if (this.route === '/logs') {
 store.startLogStream();
 } else if (this.route === '/settings') {
 this.loadConfig();
 }

 this.loaded = true;

 setInterval(() => { if (this.route === '/') store.fetchHealth(); }, 10000);
 },

 doCreateInstance() {
 const sdkNo = parseInt(this.createInstance.sdkNo) || 0;
 const port = parseInt(this.createInstance.port) || 0;
 this.store.createInstance(sdkNo, port).then(() => {
 this.createInstance.open = false;
 }).catch(() => {});
 },

  openDeviceForm(d) {
  if (d) {
  this.deviceForm = {
  open: true, editing: true, id: d.id, name: d.name, sn: d.sn,
  activation: d.activation || '', password: d.password || '0', ip: d.ip || '',
  ethernet_port: d.ethernet_port || '5005', sdk_no: d.sdk_no,
  enabled: d.enabled, online: d.online
  };
  } else {
  this.deviceForm = {
  open: true, editing: false, id: 0, name: '', sn: '', activation: '',
  password: '0', ip: '', ethernet_port: '5005', sdk_no: 0,
  enabled: 1, online: 1
  };
  }
  },
  async doSaveDevice() {
  const data = {
  name: this.deviceForm.name, sn: this.deviceForm.sn,
  activation: this.deviceForm.activation, password: this.deviceForm.password,
  ip: this.deviceForm.ip, ethernet_port: this.deviceForm.ethernet_port,
  sdk_no: this.deviceForm.sdk_no, enabled: this.deviceForm.enabled, online: this.deviceForm.online
  };
 try {
 if (this.deviceForm.editing) {
 await this.store.updateDevice(this.deviceForm.id, data);
 } else {
 await this.store.createDevice(data);
 }
 this.deviceForm.open = false;
 this.toast = { show: true, msg: 'Device saved', type: 'success' };
 setTimeout(() => { this.toast.show = false }, 3000);
 } catch (e) {}
 },

  confirmDelete(type, id) {
  const self = this;
  this.confirm = {
  show: true,
  title: 'Confirm Delete',
  msg: 'Delete ' + type + ' #' + id + '?',
  action: async () => {
  try {
  if (type === 'instance') {
  await self.store.deleteInstance(id);
  } else {
  await self.store.deleteDevice(id);
  }
  self.toast = { show: true, msg: type + ' deleted', type: 'success' };
  setTimeout(() => { self.toast.show = false }, 3000);
  } catch (e) {}
  }
  };
  },

   async toggleDeviceEnabled(d) {
   try {
   await this.store.api('/devices/' + d.id + '/toggle', 'POST');
   await this.store.fetchDevices();
   } catch (e) {}
   },

   async loadConfig() {
   try {
   const r = await fetch('/api/config');
   const data = await r.json();
   const cfg = data.find(c => c.key === 'scanlog_sync_interval');
   if (cfg) this.syncInterval = parseInt(cfg.value) || 60;
   } catch (e) {}
   },
   async saveSyncInterval() {
   try {
   const r = await fetch('/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key: 'scanlog_sync_interval', value: String(this.syncInterval) }),
   });
   if (r.ok) {
    this.toast = { show: true, msg: 'Sync interval updated', type: 'success' };
    setTimeout(() => { this.toast.show = false }, 3000);
   }
 } catch (e) {
 this.toast = { show: true, msg: 'Failed to update config', type: 'error' };
 setTimeout(() => { this.toast.show = false }, 3000);
 }
 },
 async saveUserSyncLimit() {
 try {
 const self = this;
 const r = await fetch('/api/config', {
 method: 'PUT',
 headers: { 'Content-Type': 'application/json' },
 body: JSON.stringify({ key: 'user_sync_limit', value: String(self.usersPage.syncLimit) }),
 });
 if (r.ok) {
 self.toast = { show: true, msg: 'User sync limit updated', type: 'success' };
 setTimeout(() => { self.toast.show = false }, 3000);
 }
 } catch (e) {
 this.toast = { show: true, msg: 'Failed to update config', type: 'error' };
 setTimeout(() => { this.toast.show = false }, 3000);
 }
 },
 async loadUserSyncLimit() {
 try {
 const self = this;
 const r = await fetch('/api/config');
 const data = await r.json();
 const cfg = data.find(c => c.key === 'user_sync_limit');
 if (cfg) self.usersPage.syncLimit = parseInt(cfg.value) || 30;
 } catch (e) {}
 },
 };
}
