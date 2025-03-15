# 关于gitlab-ci的项目总结
## 项目主要目标
* 通过gitlab-ci和部署的大语言模型完成自动代码审查
## 具体步骤
### 使用gitlab api与服务器部署的大语言模型相连
* 下列代码作用共有: 1.gitlab鉴权 2.读取文件 3.review到commit中
![代码](gitlab_image/2.png)
