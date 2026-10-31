## CHAPTER 06

## Software Engineering Operations

## ACRONYMS

| API    | Application Programming Interface   |
|--------|-------------------------------------|
| ATDD   | Acceptance Test Driven Development  |
| CD     | Continuous Delivery                 |
| CI     | Continuous Integration              |
| CPU    | Central Processing Unit             |
| CONOPS | Concepts of Operations              |
| DBMS   | Database Management System          |
| IaC    | Infrastructure as-Code              |
| IaaS   | Infrastructure as a Service         |
| IT     | Information technology              |
| ITIL   | IT Infrastructure Library           |
| KPI    | Key Performance Indicator           |
| MR     | Modification request                |
| MVP    | Minimum Viable Product              |
| PaaS   | Platform as a Service               |
| PR     | Problem Report                      |
| QA     | Quality Assurance                   |
| SaaS   | Software as a Service               |
| SLA    | Service-Level Agreement             |
| SRE    | Site Reliability Engineering        |
| TDD    | Test Driven Development             |

## INTRODUCTION

Software engineering operations refers to the set of activities and tasks necessary to deploy, operate and support a software application or system while preserving its integrity and stability. These activities include the deployment and configuration of the software in the targeted operational environments and the monitoring and management of the application while it is in use (until it is retired). Once the application is operational, software engineering operations must manage any defects that are uncovered, any changes made to the system software environment and hardware equipment over time, and any new user requirements that surface.

Software engineering operations is an integral part of system and software life cycle processes [3]. The Software Engineering Operations Knowledge Area (KA) is related to all other aspects of software engineering.

Specialized software and information technology (IT) operations engineers have traditionally provided and managed IT operations services. Best practices in software engineering operations were initially published by the IT infrastructure library (ITIL) and were quickly accepted by the industry. These practices were summarized and published in the IEEE 20000 standard [1].

Historically, operations and computing centers were often located in organizational silos separate from software development activities. Progressive organizations now co-locate software development, software maintenance and some software engineering operations activities (often provided as a service and often coined DevOps). Benefits of this approach are the elimination of the organizational silos that separated these software activities and the sharing of common processes and tools. The rising popularity and growing acceptance of DevOps practices [2*] and related standards [4], including an ever-evolving set of tools, reflect this trend. DevOps aims at automating and continuously evolving software engineering activities to ensure high-quality software and to satisfy consistency/standardization, known security users who demand quicker turnaround from policies, self-documentation (transparency), software engineers.

Figure 6.1. Breakdown of Topics for the Software Engineering Operations KA.

<!-- image -->

single source of truth, configuration control, In this context, the role of software engiand scalability. From an engineering perspecneers involved in software engineering opertive, the important point is that nearly anyations has significantly evolved from 2015 thing that impacts a software product directly to 2025 with the emergence of practices like or indirectly should be considered for repreInfrastructure-as-Code (IaC), Platformsentation as code. as-Code (PaC), Agile infrastructure, softTo perform software engineering operware-defined architectures/systems, and ations tasks, some organizations use the the the availability of infrastructure as a ser- concept of Platform Engineering and Site vice (IaaS) and platform as a service (PaaS) Reliability Engineering (SRE) [6] to increase solutions. Tasks traditionally performed by productivity and software quality. The role of IT infrastructure engineers are increasingly platform engineering is to build and manage automated and made available as a service, self-service platform capabilities that can enabling application developers to perform be used by software engineers to develop, software engineering operations tasks inde- deploy, and operate software applications. On pendently as part of their daily project activ- the other hand, the role of SRE is to monities. For example, application developers in itor, automate, and improve software operamany organizations can now directly use IaaS tions with respect to non-functional aspects, and PaaS to deploy applications in producincluding availability, performance, latency, tion environments and to monitor different and security. SRE is also responsible for aspects of those applications without directly change management, emergency response, involving operations engineers. capacity planning, and overall efficiency of

Having end-to-end resources and desired software systems. state configuration managed like code, using practices such as IaC and PaC, provides ventional IT operations management proAlthough many organizations still use convalue in the form of improved repeatability, cesses, this KA focuses mainly on the role of software engineers in operations in the engineering operations processes. In this role, emerging contexts of DevOps, IaC, PaC, and Agile infrastructure practices.

In this context, we identify two main software engineering roles related to operations: Operations engineer, who is responsible for developing operations services made available as a service and accessible through an application programming interface (API), and software engineer, who can use the resulting operations services (available as a service) to independently deploy and manage applications without directly involving IT operations specialists.

## BREAKDOWN OFTOPICS FOR SOFTWARE ENGINEERING OPERATIONS

The breakdown of topics for the Software Engineering Operations KA is shown in Figure 6.1.

## 1. Software Engineering Operations Fundamentals

This first section introduces the concepts and terminology that form an underlying basis for understanding the role and scope of software engineering operations.

<!-- formula-not-decoded -->

In this Guide, the term software engineering operations refers to the knowledge, skills, processes and tools used by software engineers or their organization to ensure that a software product, including IT infrastructure, system software, and application software, operates well during development, maintenance and in real conditions of operations.

an operations engineer works closely with software engineers to develop and offer operations services such as the following:

- Provisioning, deployment, configuration, and support for containers and virtual servers,
- Designing and offering on-demand services (e.g., environment on demand, versioning, continuous integration (CI) and testing, deployment, and surveillance) for use by software engineering,
- Monitoring and troubleshooting system and application software incidents by running diagnostics, documenting problems and resolutions, prioritizing problems, and assessing impact of issues,
- Performing, automating and implementing appropriate processes for security, data protection and failover procedures,
- Overseeing capacity, storage planning and database management system (DBMS) performance,
- Providing documentation and technical specifications to IT staff for planning and implementing new or upgraded IT infrastructure and system software.

ISO/IEC/IEEE 20000-1 describes the need to develop and enhance the professional competencies of operations engineers. [1, c3s3.3][3, c6s6.4.12]To achieve this goal, software organizations should address the following:

In ISO/IEC/IEEE 12207 [3], an operator is defined as an "individual or organization that performs the operations of a system." The SWEBOK Guide modifies that definition for the term operations engineer, which refers to a software engineer who executes software

- Staff recruitment: To validate job applicants' qualifications and competencies, including their professional certifications, and to identify their strengths, weaknesses and potential capabilities against the operations engineer job description, core technologies and computer languages mastered and overall experience,
- Resource planning: To staff new or expanded engineering operations services, plan the use of new technology, plan the assignment of service management staff to development project teams,

- Operations Plan and Supplier Management
- Software Availability, Continuity and Service Levels
- Development and Operational Environment
- Software Capacity Management
- Software and Data Safety, Security, Integrity, Protection and Controls
- Software Backup, Disaster Recovery and Failover
- Incident and Change Management
- Operations Support
- Monitor, Measure, Track and Review
- Service Reporting

<!-- image -->

Figure 6.2. Software Engineering Operations Processes and Activities

develop succession planning and other service delivery processes, release processes, staffing gaps created by staff turnover, control processes, resolution processes and rela· Resource training and development: tionship processes. These operations processes To identify training and development are further categorized as technical processes requirements and create a training and in ISO/IEC/IEEE 12207 [3]. Operations prodevelopment plan that meets them; also, cesses, from the perspective of a software engito provide timely, effective delivery of neer, contain the activities and tasks necessary operations services. Operations engineers to deploy, configure, operate and support an should be trained in the relevant aspects existing software system or product while preof service management (e.g., via training serving its integrity. This international standard courses, self-study, mentoring and describes four main operations process activion-the-job training), and their teamwork ties: 1) prepare for the operation: that requires and leadership skills should be developed. to define an operation strategy; 2) perform A chronological training record should the operation: which consist of operating and be maintained for each individual, with monitoring; 3) manage the results of operation: descriptions of the training provided. where anomalies are recorded and addressed; and finally 4) support the customer: which 1.2. Software Engineering Operations Processes means to give assistance and consultation to [2*, s1][3, c6s6.4.12] any user of the operations services.

ISO/IEC/IEEE 20000-1 is the reference standard that presents an overview of operations processes. It specifies requirements for the design, transition, delivery and improvement of operations services. The ISO/IEC/IEEE 20000-1 describes five main operations process groups:

Finally, ISO/IEC/IEEE 32675 [4] introduces a number of software engineering operations activities using an Agile and a minimum viable product (MVP) perspective. This standard recognizes the influence of DevOps as a set of principles and practices that enable better communication and collaboration between relevant stakeholders for the purpose of spec- delays, increase quality, and ensure a conifying, developing, continuously improving, and operating software and system products and services. These processes and activities are the responsibility of operations engineers.

sistent and stable operational environment. This is typically achieved using scripting languages, which are basic programming languages. Automating operations enables For the purpose of the SWEBOK Guide, a quicker reaction in case of a failure and, engineering operations activities can be therefore, results in less downtime and fewer grouped into three main operations processes severe incidents, as alerts are sent immedi(see Figure 6.2) that each contain a number of ately. Automating such tasks is also a good operations activities, which are described in way to ensure standardization of operations in the following sections of this chapter: an organization. It also constitutes the basis for the development of operations made avail· Operations Planning (section 2), able as a service. Refer to section 6 for further · Operations Delivery (section 3), discussion on operations tools.

- Operations Control (section 4).

Each software engineering operations process includes activities performed during the pre delivery and post delivery stages of a soft- Software engineering operations is responware project. Software engineering operations planning activities occur during the pre delivery stage. These activities are covered in oughly tested before it is released (deployed this chapter.

<!-- formula-not-decoded -->

Before a software application or update can be made available to the users (i.e. released in production), the operations engineer must selective retesting of a software application, install the software as part of its deployment. or component, to verify that the software To install the software, the engineer might to be deployed will not cause unintended have to uninstall previous versions, configure effects) play an important role in software the software for its target destination, and create the necessary directories, registry files and environment variables on the target destination. This is often done using a scripting language. The installation of the software to the appropriate locations is typically done a verification step is conducted to ensure that the operation succeeded.

<!-- formula-not-decoded -->

sible for ensuring the stability of the system. For this purpose, software must be thorin production and made available to users). Because manual testing is inefficient, errorprone and non-scalable, testing must be automated as much as possible throughout the entire software process. Also, because the time available for testing is limited, regression testing and test coverage strategies (the engineering operations.

When errors are found (in production after the software is released or during internal testing phases), software engineers and software operations engineers need to troubleshoot hardware and software incidents by running electronically, but in the case of embedded diagnostics, documenting problems and ressystems, it might require the use of a phys- olutions, prioritizing problems, and assessing ical medium. Once the software is installed, the impact of the issues. The cost — in both time and money — of repeating full testing on a major piece of software is significant. To ensure that the requested problem reports 1.4. Scripting and Automating [2*, c9] (PRs) are valid, the operations engineer should replicate and verify problems by running the As part of software engineering operations, appropriate tests. Testing certain aspects of

repetitive tasks are automated to reduce the software in production can be particularly challenging. For example, when software per- 2.1. Operations Plan and Supplier Management forms critical functions, bringing it off-line to [1, c4s4.1][3, c6s6.1] test might be difficult. Generally, testing the software in the production system context is Software engineering operations planning Testing KA provides additional information and references on testing.

challenging (sometimes impossible) and could should comprise part of the process of transrequire the use of testing techniques such as lating project requirements and the needs of canary testing and dark launches. The Software the developers and maintainers into services, and it should provide a road map for directing progress. This process often involves the products and services of suppliers that must be 1.6. Performance, Reliability and well coordinated to ensure quality service. Load Balancing [1, c6s6.2] ISO/IEC/IEEE 20000-1 describes planning activities, as well as ISO/IEC/IEEE 12207, which lists the activities operations engineers formance, reliability and load balancing early considers from human, technical and system perspectives.

<!-- formula-not-decoded -->

Software operations engineers plan for perin software projects to ensure they meet the project requirements. (See section 1.2 to 1.7 of the Software Requirements KA). A current trend is for software engineers to design and use infrastructure/operations services to adjust dynamically (e.g. scalability) the infrastructure according to the demand. Using DevOps practices enables operations engineers to anticipate these needs early and provide infrastructure services that software engineers can use and test during the development stages of a project.

## 2. Software Engineering Operations Planning

- Whereas software development typically lasts from some months to a few years, the operations phase usually lasts many years. Therefore, estimating resources is a key element of operations planning. Software engineering operations planning should begin with the decision to develop a new software product and should consider its maintenance and operations requirements early. A concept document should be developed, followed by an operations and maintenance plan [1,c7s2], and

- Scope of the operations and software maintenance,
- Adaptation of the software engineering operations process and tools,
- Identification of the software engineering operations organization,
- Estimate of software engineering operations and maintenance costs.

The next planning step suggests to develop a software engineering operations plan, or concept of operations (CONOPS). This plan should be prepared during software development and should specify how users will request software modifications and report problems or issues when the software will be operational.

This topic introduces some of the generally both should address the following: accepted techniques used in software engineering operations planning. Operations engineers must deal with a number of key issues to ensure software operates effectively. Operations engineers should document their software engineering operations steps and tools, using any type, form or medium suitable for the purpose (e.g., Wikis, documents, and more). The following topics are typically considered suitable as evidence of well documented operations:

- Policies and plans,
- Service documentation,
- Procedures,
- Processes, and
- Process control records.

Software engineering operations planning is (MR)) planning is required. Once individual addressed in ISO/IEC/IEEE 12207 [3] and requests are received and validated, the release ISO/IEC/IEEE 32675 [4]. The standards proor version planning activity requires that opervide guidelines for planning, implementing, ations engineers perform the following tasks: maintaining, automating and supporting production software. Finally, at the highest planIdentify the target availability dates of ning level, the operations organization must individual requests, conduct business planning activities (e.g., budAgree on the content of subsequent getary, financial and human resources), just as releases or versions, all the other divisions of the organization (refer Identify potential conflicts and develop to the Software Engineering Management alternatives, KA). ISO/IEC/IEEE 20000-1 recommends Assess the risk of a given release and that the operations plan addresses issues assodevelop a rollback and data migration plan ciated with a number of planning perspectives, (see section 3.3) in case problems arise, including the following: · Inform all stakeholders.

## [1, c7s3][3, c6s6.1]

- The roles and responsibilities for imple- 2.1.2. Supplier Management menting, operating and maintaining the new or changed service,

· Activities to be performed by customers Supplier management ensures that the orgaand suppliers, nization's suppliers and their performance are Changes to the existing service managemanaged appropriately to support the seamment framework and services, less provision of quality products and services. Communication to the relevant parties, ISO/IEC/IEEE 12207 lists the activities that · New or changed contracts and agreements the operations engineer will perform to estabto align with changes in business needs, lish an agreement to acquire suppliers' products Staffing and recruitment requirements, and/or services. From an operations engineer's Skills and training requirements (e.g., perspective, the nature of the relationship users, technical support), with suppliers and the approach should be Processes, measures, methods and tools determined by the nature of the products and to be used in connection with the new or services needed in a project. Managing supchanged service, pliers of services related to operational soft· Capacity management, ware includes managing out-sourced services · Financial management, and cloud services, like IaaS and PaaS.

- Budgets and timescales,
- Service acceptance criteria, and
- The expected outcomes from operating the new service, expressed in measurable terms.

## 2.2. Development and Operational Environments [2*, c9]

The overall software process requires the use of different environments at different stages. This plan ensures that an operational These are typically defined as the development strategy is defined, conditions for correct environment, the testing or quality assurance operations are identified and evaluated, the (QA) environment, the preproduction envisoftware is tested at scale to operate in its ronment, and the production environment. intended environment, and surveillance is To build quality into the product and reduce provided to ensure responsiveness and availthe risks associated with the release of softability of the software by ensuring constant ware in the production environment (whether support. At the individual request level (e.g., the release is associated with new functionproblem report (PR) or modification request ality or software defects), engineers must ensure that the different environments are all capacity, at all times, to meet current and coherent and synchronized with the produc- future agreed-upon demands created by the tion environment. customer's business needs. The current and For this reason, DevOps recommends that expected business requirements for services the creation of all the different environments should be understood in terms of what the be automated and built from a single code business needs in order to deliver its prodrepository. In mature DevOps organizations, ucts or services to its customers. Business prethe creation of the different environments is dictions and workload estimates should be completely automated and made available as translated into specific requirements and doca service. Also, all environments need to be umented. The reaction to variations in workbuilt from the same code source (single source load or environment should be predictable; of truth) to ensure that all the environments data on current and previous components, as are synchronized with the production enviwell as resource utilization at an appropriate ronment in which the software is released. level, should be captured and analyzed to supThis leads to the concept of infrastructure as port the process. code (IaC).

Capacity management is the focal point for all performance and capacity issues. The 2.3. Software Availability, Continuity, and process should directly support the developService Levels [1, c6s6.3] ment of new and changed services by sizing and modeling these services. A capacity plan Service availability and continuity must be documenting the actual performance of the managed to ensure that customer commitments infrastructure and the expected requirements are met. Because service availability and conti- should be produced at a suitable frequency (at nuity are defined as nonfunctional requirements least annually), considering the rate of change early in a project (see the Software Quality in services and service volumes, informaKA), operations engineers will ensure that tion in the change management reports, and the proper infrastructure is planned, designed, changing customer business requirements. implemented and tested. Software availability The capacity plan should document costed is measured and recorded, and unplanned options for meeting business requirements nonavailability is investigated and appropriate and recommend solutions to ensure achieveactions taken. Service reports produce availment of the agreed-upon service-level targets ability and continuity indicators of operations as defined in the SLA. The technical infraservices against service-level targets. structure and its current and projected capacities should be well understood to ensure

<!-- formula-not-decoded -->

ISO/IEC/IEEE 20000-1 also proposes that the following should be quickly available following a major service failure or disaster to ensure continuity planning and testing: backups of data, documents and software, 2.4. Software Capacity Management and any equipment or staff necessary for ser[1, c6s6.5] vice restoration. Backup and data recovery are important activities; successful recovery ISO/IEC/IEEE 20000-1 describes the need is especially vital. The need for successful to ensure that the software product has the recovery should influence which backup and

The service-level management process monitors the agreed software level of service, including optimal software operations. workload characteristics, performance and availability trend information and customer satisfaction analysis. Defining, agreeing to and documenting service-level agreements (SLAs) can help clarify the full range of operations services obligations provided. The Software Maintenance KA provides additional information and references about SLAs.

recovery methods are used (full or incremental), how frequently restore points are established, where they are stored, and how long they are retained.

- e. All staff should be made aware of the information security policy,
- f. Expert help on risk assessment and control implementation should be available,

Preparedness and regular test of backup, g. Changes should not compromise the disaster recovery, and failover should be coneffective operation of controls, and stantly rehearsed as changes to the produch. Information security incidents should tion environment are made. This is another be reported following incident manageessential activity that is triggered when outage ment procedures, and a response should assessments are done. Testing disaster recovery be initiated. requires stopping the service, identifying the checkpoint state and triggering the failover In line with the evolution of DevOps, process. Software engineers should underDevSecOps is promoting the integration stand that failure is inevitable and that autoof security early and throughout the softmated failover daemons can reduce recovery ware process, which includes the integratime drastically. To achieve this, software tion of different security mechanisms and applications should include failure-handling tools at the operations level. The goal is to logic; this must be planned during developautomate the detection and correction of ment. DevOps can help organizations that security issues as early as possible in the want to reduce failovers and disasters by autooverall process. mating and launching tests as often as possible to ensure readiness in case of a failure or cata- 3. Software Engineering Operations strophic event. Delivery

## 2.6. Software and Data Safety, Security, Integrity, Protection, and Controls

The need to manage information security effectively within all service activities is described in ISO/IEC/IEEE 20000-1. This is done by conducting a software risk assessment on the security and availability of information. Operations engineers should strive to 3.1. Operational Testing, Verification, and enforce the following controls: Acceptance [2*, c10] [3, c6s6.3.5.3d]

This topic introduces some of the generally accepted processes used during software [1, c6s6.6] engineering operations delivery (ISO/IEC/ IEEE 20000-1): SLA, service reporting, service continuity, availability management, budgeting and accounting for IT services, capacity management, and information security management.

So e   s orrt s    e tr information security policy, communiware verification as early as possible, using cate it to staff and customers, and act to test-driven development (TDD) and accepensure its effective implementation, tance test-driven development (ATDD) b. Information security management roles techniques and tools that ensure that operaand responsibilities should be defined tional testing is ongoing during the developand allocated to post holders, ment of the software, not only at the end of c. A representative of the management team a project. DevOps plays an important role in should be assigned the role of monitoring developing and automating software testing and maintaining the effectiveness of the services and integrating different tools to information security policy, improve software productivity and quality. d. Staff with significant security roles should (See TDD and ATDD in the Software receive information security training, Testing KA.)

## 3.2. Deployment/Release Engineering [2*,c12][3,c6s6.3.5.3d]

A software operations engineer's main responsibility relates to the deployment and release of software to ensure its continued performance. As defined in [2*], "deployment is the installation of a specified version of software to a given environment (e.g., deploying code into an integration test enviwhereas "release is when we make a feature (or set of features) available to all our customers or a segment of customers (e.g., we enable the feature to be used by 5% of our customer base)." Release processes include all the activities related to release management. ISO/IEC/IEEE 12207 [3] lists release control activities and explains the need to identify and record release requests, identify the software system elements in a release followed by approval, and track the releases in their specified environments.

staging environment to support the release of a new version of an application. In other words, the basic strategy involves deploying the new version of the application to a staging environment. Application-based release strategies are based on the use of toggles (e.g., feature toggles) that make it possible to enable or disable specific sections of the code (e.g., a feature) using configuration parameters.

Deployment and release are supported ronment or deploying code in production)," by automation techniques and tools. The canary release testing technique is a partial and time-limited deployment of a change in a service and an evaluation of that change. This evaluation helps the operations engineer decide whether to proceed with a complete deployment. Similarly, tools that manage the installation of new software typically observe the newly started software for a while, ensuring that the software doesn't crash or otherwise misbehave. The same technique is useful for observing recent changes; if they do not pass the validation period, they can be automatically rolled back. The Software Configuration Management KA provides more information about the release processes. Once the application platform is deployed in the targeted production environment, the decision to make it available to the users (release it) becomes a business decision.

## 3.3. Rollback and Data Migration

[2*, c12][3, c6s6.4.10.3]

Rollback and data migration are terms used to describe the process of returning software and its database to a state where they work properly. Software engineers ensure that when a new version of the software and its dataware on the different servers, launching the bases have been modified and deployed to production, they can easily and quickly be rolled back in case the new version is causing Different release engineering stratedefects or product degradation in production. gies can be used to reduce the risks assoThis means a planned and rehearsed rollback ciated with software releases. These is done before a new version of the software strategies can be grouped into two main catis deployed in production. DevOps processes egories: environment-based release strateautomate this process to make it faster; in fact, -Sue s  us e ro ss ur -e e sro Environment-based release strategies use a back and data migration to a previous state so

DevOps advocates integrating development and operations in the same team to improve software engineering operations efficiency. In traditional software processes, when an application is ready for deployment, it is transferred from a development team to an operations team that is responsible for deployment, which is mostly done manually. This results in processes that are inefficient from both a time and a quality perspective. To improve the efficiency of the deployment process, DevOps calls for automating the different deployment steps, including packaging the code, generating configuration files, restarting the servers, configuring the servers and databases, installing the softexecution of the application, and executing smoke testing.

quickly that the end user doesn't notice that and reviewed in a controlled manner. All release — can be used to support rollback.

there was a problem. Both release strategy change requests are recorded and classified categories (described in section 3.2) — envi- (e.g., emergency, urgent, major and minor). ron   s  ass as as ao-e a ase a-ges and the need for a rollback strategy in case of failure. Large systems might require that a 3.4. Problem Resolution [1, c8s8.3] change schedule be planned with the product manager and end users.

The objective of this operations process is to Whereas in traditional software delivery minimize disruption to the business through processes (or software life cycle models), all the identification and analysis of the cause of changes are delivered as part of new softsoftware and system incidents and problems. ware releases (containing multiple changes This approach may require the involvement related to different aspects of the application of a multidisciplinary team, whose software or system) issued at fixed time intervals (e.g., engineers and operations engineers investievery three months), DevOps aims to deliver gate, for example, recurring production probsmall units of change (a single new functionlems that might have an underlying cause in ality or service, or defect fix, rather than a new software infrastructure and system compoversion of an application containing multiple nents. This might require monitoring, logging changes) on demand and independently from and profiling the software and its infrastruceach other. For this purpose, software applicature behavior. tions (or services) must be architected to enable small, independent software deployments.

## 4. Software Engineering Operations Control

This topic introduces some generally accepted techniques used in software engineering operations control.

<!-- formula-not-decoded -->

<!-- formula-not-decoded -->

Software engineering operations activities monitor capacity, continuity and availability. In a DevOps mindset, hope should [1,c8s8.2] not be a strategy; instead, engineers should be informed about system quality and operaIncident management is the process of tional health with evidence, such as the folrecording, prioritizing and assessing the lowing key performance indicators (KPI), business impact, resolution, escalation and which are available to stakeholders in closure of software incidents. The modern real time: DevOps approach automates software surveillance using alerts and logs to prevent · Production system's monitoring and minor incidents from becoming major inciproduct telemetry, dents. When an incident occurs, proper analActionable verification and validaysis and/or post mortems must be conducted tion results before and after release to to find the source of the incident and approproduction, priate solutions must be implemented to pre· End-user activity and resource use, vent similar incidents to happen again in · Impact analysis results, the future.

<!-- formula-not-decoded -->

This operations process ensures that all changes are assessed, approved, implemented

- Inter- and intra-related dependencies required for system operation,
- Configuration changes unrelated to approved deployment tasks, and
- Security and resilience performance capability.

## 4.4. Operations Support

ISO/IEC/IEEE 12207 [3], ISO/IEC/ IEEE 20000-1 [1] and ISO/IEC/IEEE The overall operations process needs to be 32675 [4] identify the primary software automated as much as possible to prevent inciengineering operations activities that supdents and problems, and automated testing port the operations processes — activities needs to be integrated throughout the process. -l  s o tur ot s t  ops   le intended environment — and the primary mented with proper analytics techniques to activities that provide support to the cusdetect problems as early as possible to prevent tomers of the software products. Operations incidents. For this purpose, data collected support activities are initiated at the planat all layers of the product stack (including ning stage of the project and are then exeapplication layer, operating system layer and cuted, which often requires techniques and infrastructure layer) must be collected and tools to proactively monitor the product analyzed. Using product telemetry not only and services and react quickly to events allows engineers to detect potential issues but and incidents. Support activities are often also provides the foundation for identifying described in SLAs. the source of the problem.

<!-- formula-not-decoded -->

[2*, c7]

<!-- formula-not-decoded -->

[3, c6s6.4.12.3c4]

Service reporting aims to produce agreedupon, timely, reliable and accurate informaOperations engineers must manage a number tion for decision-making. Each service report of risks. IEEE 2675 [4] defines continuous helps demonstrate how an operations serrisk management as a continuous process that vice has performed and whether it has met can be automated to monitor operations consome stated and agreed-upon end-user objecstantly for risks that can affect software availtive. Typical service reports address perfor- ability, scalability and security. Operations mance against service-level targets, as well engineers can take measures to automate as security breaches, the volume of transacthe alerts. To decide what events will trigger tions and resource use, incidents and failures, an alert, they need to talk with product trend information, and satisfaction analysis. owners and software engineers to establish Operations engineers need to establish autoan agreed-upon level of risk tolerance. Other mated systems and tools for measurement to perspectives are to choose the deployment do the following: process that is appropriate for the risk profile of a given service and the risks of exposing

- Determine whether measures are already private data. available or additional instrumentation S os Sttg ag ors n ltns s ing is needed,
- Select or develop a framework and tools to allow coordination of measurement collection for analysis, reporting and control.

## 5. Practical Considerations

This topic introduces practical considerationss for software engineering operations.

<!-- formula-not-decoded -->

Automation is playing an important role in modern software operations. Software engineers achieve the best results when coupling applications and operations automation. Although automation primarily focuses on managing the life cycle of a system or infrastructure (e.g., user account creation, environments and server provisioning, runtime config changes), it can also be useful in other Continuous delivery (CD) is a software uuaae sus as rs te a  a sees rs  tes to help software engineers deploy, test and tools to provide frequent releases of new sysdebug during development. Trends in opertems (including software) to staging or varations automation aim to reduce complexity, ious test environments. CD continuously accelerate provisioning of infrastructure, assembles the latest code and configuration offer operations services scripts to developers, from the head into release candidates. define applications, automate deployment and test workflows.

## 5.4. Software Engineering Operations for Small Organizations

Very small organizations (organizations of up to 25 people) have difficulty applying standa      sa tions, as their requirements can overwhelm Continuous deployment (aka CD) is an autothe capabilities of small organizations. This mated process of deploying changes to prois where the ISO/IEC 29110 series of standuction by verifying intended features and dards is useful, as it provides standards and validations to reduce risk. Jez Humble and guidelines adapted to very small organiDavid Farley [8] pointed out that "[t]he biggest zations to ensure the quality of their softrisk to any software effort is that you end up ware engineering operations [7]. Software building something that isn't useful. The earengineers should be aware that operations lier and more frequently you get working softprocesses can be adapted to small organizaware in front of real users, the quicker you get tions and that the ISO/IEC CD 29110-5-5 feedback to find out how valuable it really is." will be addressing this purpose.

Continuous testing is a software testing practice that involves testing the software at every stage of the software development life cycle. Continuous testing aims to evaluate the quality of software at every step of the CD process by testing early and often. Continuous testing involves various stakeholders, such as developers, DevOps personnel, and QA and end-users.

## 6.1. Containers and Virtualization

## 6. Software Engineering Operations Tools

[1, c5s5g][2*, c12] Different container/virtualization technologies and management tools (also called orchestrators) are available to operations engineers to improve the scalability of applications and standardize software deployment across multiple computer and server suppliers. [4, c6s6.4.12] Operations engineers use their knowledge of the size and complexity of each project to identify the best tool for flexibility, security and monitoring.

<!-- formula-not-decoded -->

Different technologies and tools can be used to manage software deployments in different environments. [4, c5s5.1] Also, different tools are usually combined to cover the different phases and aspects of software deployment, ranging from the specification of deployment and configuration using descriptor files to the

This topic encompasses tools that are particularly important in software engineering operations for maximizing the efficient use of personnel. Automating development, maintenance and operations-related tasks saves engineering resources and improves quality and turnaround. When implemented appropriately, such automated tasks are generally faster, easier and more reliable than they would be if they were attempted manually by software engineers and operations engineers. DevOps supports such automation for integrating, building, packaging, configuring, and deploying reliable and secure systems. It combines development, maintenance, and operations resources and procedures to perform CI, delivery, testing and deployment.

automated deployment and management of collect data at all layers of the software system production environment resources.

(including application, operating system and server) and extract information that can be 6.3. Automated Test [2*, c10] used to analyze and monitor different aspects of the system to detect issues and follow To enable fast and constant feedback to the the evolution of various properties. James developers, testing must be automated as Turnbull [9] describes a general monitoring much as possible throughout the entire soft- framework architecture used by engineering ware delivery process, including throughout operations in many technology organizations. development and operations. For this pur- Implementing monitoring solutions requires pose, a testing strategy covering the different combining different techniques and tools to types of test (unit test, integration test, system collect data at different layers. This includes test, user acceptance test) must be defined, and logs at the application level, execution traces tools to support and automate the different at the operating system level and resource testing phases must be selected. The automause information (like CPU and memory use) tion of testing is critical to provide continuous at the server level. Then, based on the colfeedback to software engineers developing lected data, different analytics techniques code and thereby to improve software quality. (e.g., statistical analysis and machine learning techniques) can be used to extract relevant 6.4. Monitoring and Telemetry [2*, c14-15] information. Finally, dashboards can be used to visualize the extracted information; difMonitoring and telemetry are key aspects ferent dashboards can be developed to display

of software engineering operations. They relevant information to different stakeholders.

## MATRIX OFTOPICS VS. REFERENCE MATERIAL

|                                                    | ISO 20000-1 [1]   | The DevOps Handbook [2*]   | ISO 12207 [3]   |
|----------------------------------------------------|-------------------|----------------------------|-----------------|
| 1. Software Engineering Operations Fundamentals    |                   |                            |                 |
| 1.1. Definition of Software Engineering Operations | c3s3.3            |                            | c6s6.4.12       |
| 1.2. Software Engineering Operations Processes     | s1                |                            | c6s6.4.12       |
| 1.3. Software Installation                         | c3, c6s6.2        | c3s3.1                     |                 |
| 1.4. Scripting and Automating                      |                   | c9                         |                 |
| 1.5. Effective Testing and Troubleshooting         |                   | c3                         |                 |
| 1.6. Performance, Reliability and Load Balancing   | c6s6.2            |                            |                 |
| 2. Software Engineering Operations Planning        |                   |                            |                 |
| 2.1. Operations Plan and Supplier Management       | c4s4.1            |                            | c6s6.1          |
| 2.2. Development and Operational Environments      |                   | c9                         |                 |

| 2.3. Software Availability, Continuity and Service Levels                   | c6s6.3    |        |             |
|-----------------------------------------------------------------------------|-----------|--------|-------------|
| 2.4. Software Capacity Management                                           | c6s6.5    |        |             |
| 2.5. Software Backup, Disaster Recovery and Failover                        | c6s6.3.4  |        |             |
| 2.6. Software and Data Safety, Security, Integrity, Protection and Controls | c6s6.6    |        |             |
| 3. Software Engineering Operations Delivery                                 |           |        |             |
| 3.1. Operational Testing, Verification and Acceptance                       |           | c10    | c6s6.3.5.3d |
| 3.2. Deployment/Release Engineering                                         |           | c12    |             |
| 3.3. Rollback and                                                           |           |        |             |
| Data Migration                                                              |           |        |             |
| 3.4. Change Management                                                      | c9s9.2    |        |             |
| 3.5. Problem Management                                                     | c8s8.3    |        |             |
| 4. Software Engineering Operations Control                                  |           |        |             |
| 4.1. Incident Management                                                    | c8s8.2    |        |             |
| 4.2. Monitor, Measure, Track and Review                                     |           | c14-15 |             |
| 4.3. Operations Support                                                     | c6, c14s5 |        |             |
| 4.4. Operations Service Reporting                                           | c6s6.2    |        |             |
| 5. Practical Considerations                                                 |           |        |             |
| 5.1. Incident and Problem Prevention                                        |           | c7     |             |
| 5.2. Operational Risk Management                                            |           |        | c6s6.4.12.3 |
| 5.3. Automating Software Engineering Operations                             |           | c8     |             |
| 5.4. Software Engineering Operations for Small Organizations                |           |        |             |
| 6. Software Engineering Operations Tools                                    | c5s5g     | c12    |             |
| 6.1. Containers and Virtualization                                          |           |        |             |
| 6.2. Deployment                                                             |           | c12    |             |
| 6.3. Automated Test                                                         |           | c10    |             |
| 6.4. Monitoring and Telemetry                                               |           | c14-15 |             |

## REFERENCES

- IEEE standard, ISO/IEC/IEEE 20000[1] 1:2013, Information technology — Service

management — Part 1: Service management systems requirements, ed. IEEE, 2013.

[2*] G. Kim, J. Humble, J. Debois, J. Willis, and N. Forsgren, The DevOps Handbook: How to create world-class agility, reliability and security in technology organizations, 2nd ed., IT Revolution Press, 2021.

- [3] IEEE standard, ISO/IEC/IEEE 12207:2017, Systems and software engineering — Software Life Cycle Processes, ed. IEEE, 2017.
- [4] IEEE standard, ISO/IEC/IEEE 32675:2022, Information Technology — DevOps: Building Reliable and Secure Systems Including Application Build, Package and Deployment, ed. IEEE, 2022.
3. [5]"ISO/IEC/IEEE 24765:2017 Systems and Software Engineering
4. — Vocabulary," 2nd ed. 2017
- [6] B. Beyer, C. Jones, J. Petoff, and N.R. Murphy, Site Reliability Engineering — How Google Runs Production Systems, O'Reilly Media, 2016.
- [7] ISO/IEC CD 29110-5-5:2023, Systems and software engineering — Lifecycle profiles for Very Small Entities (VSEs), Part 5-5: Agile/DevOps guidelines.
- [8] J. Humble and D. Farley. Continuous delivery: reliable software releases through build, test, and deployment automation. Pearson Education, 2010.
- [9] J. Turnbull, The Art of Monitoring. James Turnbull, 2016.