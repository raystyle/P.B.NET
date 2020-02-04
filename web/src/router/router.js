const login = () => import('../page/login/login');
const map   = () => import('../page/map/map');
const node  = () => import('../page/node/node');

export default [
  {path: '/'     , redirect: '/map'},
  {path: '/login', component: login},
  {path: '/map'  , component: map},
  {path: '/node' , component: node},
  {path: '*'     , redirect: '/map'},
]