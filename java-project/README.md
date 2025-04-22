# 基于SpringBoot前后端分离可商城项目
## 依赖版本
### 后端
* SpringBoot 3
* Java 17
* MyBatis 3.0+
### 前端
* Vite+Vue
* Element-plus
* Axios 
## 实现过程
### SpringSecurit
#### 用户认证
* Security配置类
```java
block
```
* UserDetailService :每一个UserDetails就代表一个用户信息，其中包含用户的用户名和密码以及角色,只有当build完该类时，登录才被Security代理
```java
public class AuthorizeService implements UserDetailsService {
    @Resource
    UserMapper mapper;
    @Override
    public UserDetails loadUserByUsername(String username) throws UsernameNotFoundException {
        if(username == null){
            throw new UsernameNotFoundException("用户名不能为空");
        }
        Account account = mapper.findAccountByNameOrEmail(username);
        if (account == null){
            throw new UsernameNotFoundException("用户名或密码错误");
        }
        return User
                .withUsername(account.getUsername())
                .password(account.getPassword())
                .roles("user")
                .build();
    }
}
```
#### 用户授权
*
### 持久层设计
* tb_account表: 用户表

| Syntax      | Type        |
| ----------- | ----------- |
| id          | int         |
| username    | varchar     |
| password    | varchar     |
| role        | nosinged tinyint  |
| email        | varchar     |
| createtime  | date        |
| updatetime  | date        |

* tb_log表: 日志表

| Syntax      | Type        |
| ----------- | ----------- |
| id          | int         |
| user_id    | int     |
| work_time    | date     |
| returnvalue       | varchar  |
| coast_time        | date    |


### Redis缓存
* 使用RedisTemplate接收UserDetailService
···java
    @Autowired
    private RedisTemplate<String, UserDetails> redisTemplate;
···
···java
    public UserDetails getUserDetails(String username) {
        String key = "user:" + username; // 定义 Redis key
        UserDetails userDetails = redisTemplate.opsForValue().get(key);

        if (userDetails == null) {
            userDetails = userDetailsService.loadUserByUsername(username); // 从数据库加载
            if (userDetails != null) {
                redisTemplate.opsForValue().set(key, userDetails, Duration.ofMinutes(30)); // 设置缓存，过期时间为 30 分钟
            }
        }
        return userDetails;
    }

    public void evictUser(String username){
        String key = "user:" + username;
        redisTemplate.delete(key);
    }
···
### AOP日志
* 切入点
### 前端
#### 配置
* element自动导入
* 安装插件
```shell
npm install -D unplugin-vue-components unplugin-auto-import
```
* vite.config.js配置
```js
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(),
    AutoImport({
      resolvers: [ElementPlusResolver()],
    }),
    Components({
      resolvers: [ElementPlusResolver()],
    }),],
})
```

