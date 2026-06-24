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
 _navToken: 0,

 isDeviceDetail() {
 return this.route.startsWith('/devices/') && this.route !== '/devices';
 },

 init() {
 const self = this;
 const store = Alpine.reactive({
 instances: [],
 devices: [],
 jobs: [],
 logs: [],
 health: { total: 0, running: 0, stopped: 0, error: 0, instances: [] },

 async api(path, method, body) {
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
 async getDeviceBySN(sn, token) {
 const devices = await this.api('/devices', 'GET') || [];
 if (self._navToken !== token) return;
 const d = devices.find(d => d.sn === sn || d.sn === decodeURIComponent(sn));
 if (!d) throw new Error('Device not found: ' + sn);
 if (self._navToken !== token) return;
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
 });

 this.store = store;

 this.loaded = true;
 this.store.startLogStream();
 this.navigate(window.location.hash || '#/');
 window.addEventListener('hashchange', () => {
 this.navigate(window.location.hash);
 });

 setInterval(() => { if (this.route === '/') store.fetchHealth(); }, 10000);
 },

 navigate(hash) {
 const path = hash.replace('#', '') || '/';
 this.route = path;
 this.opResult = '';
 this.deviceDetail = {};
 this._navToken++;

 if (path === '/') {
 this.store.fetchHealth();
 } else if (path === '/instances') {
 this.store.fetchInstances();
 } else if (path === '/devices') {
 this.store.fetchDevices();
 } else if (path.startsWith('/devices/')) {
 const sn = path.split('/')[2];
 if (sn) this.store.getDeviceBySN(sn, this._navToken);
 } else if (path === '/jobs') {
 this.store.fetchJobs();
 this.store.fetchInstances();
 }
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
  };
}
