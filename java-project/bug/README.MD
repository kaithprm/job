# 问题与解决过程
## 1.fastjson转码失效问题
### 问题描述: 使用ObjectJson.toString时不显示正确返回值，只显示空json
### 问题状态: 已解决
### 问题原因: lombok的 @Data 注解没有生效 导致没有get方法
### 解决方法: spring initializr构建时没有写lombok的version 将dependcy和plug指定版本即可
