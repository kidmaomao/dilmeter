// Test-only mount of the production favorite editor. Not an application entry.
import { createApp, h } from 'vue';
import { createVuetify } from 'vuetify';
import { VApp } from 'vuetify/components';
import 'vuetify/styles';
import HealerMonitorPanel from '../src/components/HealerMonitorPanel.vue';
createApp({render:()=>h(VApp,null,{default:()=>h(HealerMonitorPanel,{embedded:true,open:true,isRecordReplay:false})})}).use(createVuetify()).mount('#app');
