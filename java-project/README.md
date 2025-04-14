# 基于SpringBoot前后端分离可二次开发模板项目
## 依赖版本
### 后端
* SpringBoot 3
* Java 17
* MyBatis 3.0+
### 前端
* Vite+Vue
* Element-plus 
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
