# Open Source Summit Korea (~ 04.26 23:59)

### Session Title
k8s 위에서 DevSecOps 부터 IDP 하루 만에 시작하기: Nullus 프로젝트 소개

### Description (~1200 characters)
```sh
# 안내
# For information on suggested topics and other important details, please review the CFP Guide: https://events.linuxfoundation.org/open-source-summit-korea/program/cfp/

# The Linux Foundation collects and uses the information below to help us in the speaker evaluation process and to create a diverse schedule.

# At The Linux Foundation, we are committed to safeguarding your privacy. Sensitive speaker information will only be accessible to event organizers and program committee members who adhere to the highest confidentiality standards. Rest assured that your information will never be sold or shared beyond these parties.

# Speaker personal information including name, company, job title, biography and photo will appear on the public schedule. For information on our privacy practices and commitment to protecting your privacy, please review our Privacy Policy.
# https://www.linuxfoundation.org/legal/privacy-policy

# You can update or delete your account information at any time through your Sessionize profile account settings. Please contact support@sessionize.com directly for any questions or issues. For any other questions, please contact us directly at cfp@linuxfoundation.org.
```

Nullus는 Kubernetes 기반 DevSecOps 자동화 오픈소스 플랫폼으로, 플랫폼 엔지니어가 반복적으로 겪는 "도구 선택 혼란, 버전 호환성 문제, 긴 초기 구축 시간"을 줄이기 위해 만들어졌습니다. 이 세션에서는 특정 제품 홍보가 아니라, 다양한 조직이 재사용할 수 있는 클라우드 네이티브 운영 패턴을 공유합니다.

발표에서는 세 가지를 다룹니다. 첫째, Best Practice 템플릿(예: GitHub/GitLab + CI + Argo CD + Observability)으로 선택 비용을 줄이는 방법. 둘째, 노코드 설정 UI와 3-Phase 설치 오케스트레이션(기반 인프라 -> 플랫폼 앱 -> 연동)을 통해 실패 확률을 낮추는 방법. 셋째, Known Issues 카탈로그, 호환성 매트릭스, OIDC/RBAC 연계를 통해 Day 0 설치부터 Day 2 운영까지 이어지는 안정화 전략입니다.

참가자는 세션 후 아래를 가져갈 수 있습니다: (1) 팀 규모와 숙련도가 달라도 적용 가능한 플랫폼 부트스트랩 청사진, (2) Helm/Kubernetes 배포 실패를 줄이기 위한 실전 체크리스트, (3) DevOps 팀과 개발팀이 함께 사용할 수 있는 셀프서비스 운영 모델. 클라우드/오케스트레이션 입문자도 이해할 수 있도록 구조와 의사결정 기준을 중심으로 설명합니다.

### Which track are you submitting for?
Cloud & Orchestration

### Cloud & Orchestration Topic
Cloud Native Application Development and Operations

### Session Format
Session Presentation (30-40 minutes in length)

### Audience Level
Beginner

### Benefits to the Ecosystem
이 세션은 "플랫폼을 처음 구축하는 팀"이 바로 활용할 수 있는 오픈 패턴을 공유합니다. 특정 벤더나 폐쇄형 솔루션이 아니라 Kubernetes/Helm/OIDC/GitOps 생태계에서 재사용 가능한 설계 원칙, 실패 복구 전략, 운영 체크리스트를 공개해 커뮤니티의 시행착오를 줄이는 데 기여합니다. 또한 Best Practiceh와 호환성 관리 방식을 통해 프로젝트 간 지식 이전을 쉽게 만들고, 중소규모 팀도 클라우드 네이티브 운영 수준을 빠르게 끌어올릴 수 있도록 돕습니다.

### Presented this talk before?
Yes

### Conference Name
Korea, CloudBro Ship to Production

### Presentation Month and Year
2026년 4월
https://www.youtube.com/watch?v=jn4PuNYVlhQ&t=439s

### Which language will you use for your presentation?
Korean

### Interested in Speaker Office Hours?
Yes

### We offer attendees the opportunity to meet with expert speakers during dedicated office hours. These sessions are designed for attendees to ask questions, engage in discussions, and gain deeper insights on specialized topics.

### What topics would you be available to discuss?
Please provide a brief description of what you could cover and why attendees might find it valuable. (e.g., in-depth technical guidance, best practices, career advice, emerging trends, etc.)

Kubernetes 기반 DevSecOps 플랫폼의 초기 설계와 운영 전환(설치 -> 검증 -> 운영)을 주제로 심화 논의가 가능합니다. 특히 Golden Path 템플릿 설계 방법, Helm 배포 실패 시 롤백/재시도 전략, OIDC 기반 역할 모델(Admin/DevOps/Developer) 적용, 호환성 매트릭스/Known Issues 운영 방식, 그리고 소규모 팀이 "Day 0 자동화"를 현실적으로 도입하는 방법을 다룰 수 있습니다. 참석자는 즉시 적용 가능한 체크리스트와 팀 내 의사결정 프레임을 얻을 수 있습니다.

