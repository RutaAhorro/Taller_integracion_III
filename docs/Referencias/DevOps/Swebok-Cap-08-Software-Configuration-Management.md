## CHAPTER 08

## Software Configuration Management

## ACRONYMS

| CR   | Change Request                                                         |
|------|------------------------------------------------------------------------|
| CCB  | Configuration Control Board                                            |
| CI   | Configuration Item                                                     |
| CM   | Configuration Management                                               |
| CMDB | Configuration Management Database                                      |
| CMMI | Software Engineering Institute's Capability Maturity Model Integration |
| FCA  | Functional Configuration Audit                                         |
| MBX  | Model Based Experience                                                 |
| PCA  | Physical Configuration Audit                                           |
| QA   | Quality Assurance                                                      |
| SBOM | Software Bill Of Materials                                             |
| SCCB | Software Configuration Control Board                                   |
| SCR  | Software Change Request                                                |
| SCI  | Software Configuration Item                                            |
| SCM  | Software Configuration Management                                      |
| SCMP | Software Configuration Management Plan                                 |
| SCSA | Software Configuration Status Accounting                               |
| SLCP | Software Life Cycle Process                                            |
| SQA  | Software Quality Assurance                                             |
| V&V  | Verification And Validation                                            |
| VDD  | Version Description Document                                           |

## INTRODUCTION

Software configuration management (SCM) is formally defined as "the process of applying configuration management [CM] throughout the software life cycle to ensure the completeness and correctness of CIs [configuration items]," with CM defined as "a discipline applying technical and administrative direction and surveillance to identify and document the functional and physical characteristics of a configuration item, control changes to those characteristics, record and report change processing and implementation status, and verify compliance with specified requirements" [1]. SCM is a software life cycle process (SLCP) that supports project management, development and maintenance activities, quality assurance (QA) activities, and the customers and users of the end product.

The concepts of CM apply to all items controlled, although some differences exist between implementing hardware CM and implementing software CM. CM applies equally to iterative and incremental software development methodology.

SCM is closely related to software quality assurance (SQA). As defined in the Software Quality KA, SQA processes ensure that the sofes p s posscs pe spros precs life cycle conform to their specified requirements by requiring software engineers to plan, enact and perform a set of activities that demonstrate that those specifications are built into the software. SCM activities support these SQA goals through software configuration activities (presented later in this chapter). The configuration audit activity can be described as a review of CIs and is closely related to the reviews defined in the quality plan.

The SCM activities should operationalize SCM process management and planning, software configuration identification, software configuration change control, software configuration status accounting (SCSA),

Figure 8.1. Breakdown of Topics for the Software Configuration Management KA.

<!-- image -->

## [2, c6, c7]

SCM controls the evolution and integrity of 1. Determines what is expected to be under a product by identifying its elements (known control during project development as CIs); managing and controlling change; 2. Identifies and records who developed what and verifying, recording and reporting on CI as well as when and where it is allocated configuration information. From the soft3. Allows controlled changes ware engineer's perspective, SCM facilitates 4. Tracks CIs' relationships to show development and change implementation how changes that affect one CI might activities. Successful SCM implementation affect other CIs requires careful planning and management, 5. Keeps CI versions under control which requires a strong understanding of 6. Ensures that the quality of the CIs delivered the organizational context for, and the conmeets the requirements for intended use straints placed on, the design and implementation of the SCM process. The SCM The SCM KA is related to all other KAs plan can be developed once for the organibecause SCM's object is the artifact produced zation and then adjusted as needed for indisoftware configuration auditing, and soft- 1. Management of the SCM Process ware release management and delivery. This operationalization:

and used throughout the software engi- vidual projects. neering process.

## BREAKDOWN OF TOPICS FOR SOFTWARE CONFIGURATION MANAGEMENT

<!-- formula-not-decoded -->

[4*, c25]

To plan an SCM process for a project, it is necessary to understand the organizational Figure 8.1 shows the breakdown of topics for context and the relationships among orgathe SCM KA. nizational elements. SCM interacts not just with the particular project but also with sev- the contract between the acquirer and the eral other areas of the organization. supplier might contain provisions affecting The organizational elements responsible the SCM process (e.g., certain configurafor software engineering supporting protion audits might be required, or the contract cesses might be structured in various ways. might specify that certain items be placed The overall responsibility for SCM often under CM). When the software to be develrests with a distinct part of the organization oped could affect public safety, external reguor with a designated individual. However, latory bodies may impose constraints. Finally, responsibility for certain SCM tasks might the SLCP chosen for a software project and be assigned to other parts of the organization the level of formalism selected for imple(such as the development division). mentation will also affect SLCP design and implementation.

Software engineers can also find guidance for designing and implementing an ities take place in parallel with hardware and SCM process in "best practice," as reflected firmware CM activities and must be con- in the software engineering standards issued sistent with system-level CM. Note that by the various standards organizations. (See firmware contains hardware and software; Appendix B for more information about these

<!-- formula-not-decoded -->

<!-- formula-not-decoded -->

SCM process planning for a project should gram might also be under SCM control. The be consistent with the organizational context, o cg  coqdnns o y s s   t guidance and the nature of the project (e.g., size, safety criticality and security). The major activities covered in the plan are software conPerhaps the closest relationship is with figuration identification, software configurathe software development and maintenance tion control, SCSA, software configuration organizations. It is within this context that auditing, and software release management many of the software configuration control and delivery. In addition, issues such as orgatasks are conducted. Frequently, the same nization and responsibilities, resources and tools support development, maintenance, and schedules, tool selection and implementaSCM purposes. tion, vendor and subcontractor control, and interface control are typically considered. The planning activity's results are recorded in an SCM plan (SCMP), which is subject to SQA review and audit.

SCM might interface with an organiza- 1.3 Planning for SCM tion's QA activity on issues such as records management and nonconforming items. Regarding the former, project records subject to provisions of the organization's QA prononconforming items. However, SCM might assist with tracking and reporting on software configuration items (SCIs) in this category.

Software is frequently developed as part i of a larger system containing hardware and firmware elements. In this case, SCM activtherefore, both hardware and software CM standards.) concepts apply.

<!-- formula-not-decoded -->

the SCM process come from many sources. Policies and procedures set forth at corporate or other organizational levels might influence the SCM process for a project. In addition, one person changes a CI. There are many

Branching and merging strategies should be carefully planned and communicated Constraints affecting, and guidance for, because they affect many SCM activities. SCM defines a branch as a set of evolving source file versions [1]. Merging consists of combining different changes to the same file or prescribe the design and implementation of [1]. This typically occurs when more than branching and merging strategies in common should be carefully planned. The following use. (See the Further Readings section for questions should be considered: additional discussion.)

The software development life cycle model chosen (see Software Life Cycle Models in the Software Engineering Process KA) also affects SCM activities, and SCM planning should consider this. For instance, many software development approaches use continuous integration, which is characterized by frequent build-test-deploy cycles. Clearly, SCM activities must be planned accordingly.

<!-- formula-not-decoded -->

Organizational roles must be clearly identified to prevent confusion about who will perform specific SCM activities or tasks. These responsibilities must also be assigned to organizational entities; this can be made clear by the responsible individual's title or by designating the organizational division or section in addition to the individual responsible within that section. The overall authority and reporting channels for SCM should also be identified, although this might be accomplished at the project management or the QA planning stage.

<!-- formula-not-decoded -->

- Organization: What motivates tool acquisition from an organizational perspective?
- Tools: Can we use commercial tools, or do we need to develop our own tools specifically for this project?
- Environment: What constraints are imposed by the organization and its technical context?
- Legacy: How will projects use (or not use) the new tools?
- Financing: Who will pay for the tools' acquisition, maintenance, training and customization?
- Scope: How will the new tools be deployed — for instance, through the entire organization or only on specific projects?
- Ownership: Who is responsible for introducing new tools?
- Future: What is the plan for the tools' use in the future?
- Change: How adaptable are the tools?
- Branching and merging: Are the tools' capabilities compatible with planned branching and merging strategies?
- Integration: Do the various SCM tools integrate among themselves? Do they integrate with other tools in use in the organization?
- Migration: Can the repository maintained by the version control tool be ported to another version control tool while maintaining the complete history of the CIs it contains?

SCM requires a set of tools instead of a single tool. Such tool sets are sometimes tion planning effort, the team must determine whether the SCM workbench will be open (tools from different suppliers will be used in different SCM process activities) or integrated (elements of the workbench are designed to work together).

Planning for SCM identifies the resources — including staff and tools — involved in carrying out SCM activities and tasks. It also identifies the necessary sequences of SCM tasks and establishes each task's place in the project schedule and its position relative to milestones established at the project called workbenches. As part of the tool selecmanagement planning stage. Any training requirements for implementing the plans and training new staff members are also specified.

<!-- formula-not-decoded -->

Organization size and the type of projects as a a as  ae    aor  a   sSe

selection and implementation of SCM tools SCM Tools, section 7 of this document)

<!-- formula-not-decoded -->

which branching strategies will be used, how [2, c13] [3*, c13s9-c14s2] frequently builds will occur, how often automated tests of all kinds will be run).

A software project might acquire or use purGuidance on creating and maintaining an chased software products, such as compilers or SCMP, based on the information produced o s a is         s ns and how these items will be managed with number of sources, such as [2]. This reference configuration control (e.g., integrated into the provides requirements for information to be project libraries) and how changes or updates contained in an SCMP. An SCMP should will be evaluated and managed. include the following sections:

Similar considerations apply to subcontracted software. When a project uses subcontracted software, both the SCM requirements to be imposed on the subcontractor's SCM process and the means for monitoring compliance need to be established. The latter includes determining what SCM information must be available for effective compliance monitoring.

## 1.3.5 Interface Control

- Introduction (purpose, scope, terms used)
- SCM Management (organization, responsibilities, authorities, applicable policies, directives, procedures)
- SCM Activities (configuration identification, configuration control, etc.)
- SCM Schedules (coordination with other project activities)
- SCM Resources (tools, physical resources and human resources)
- SCMP Maintenance

When a software item interfaces with another software or with a hardware item, a change 1.5 Monitoring of Software Configuration to either item can affect the other. Planning Management for the SCM process considers how the inter[3*, c11s3] facing items will be identified and how changes to the items will be managed and communicated. The SCM role may be part of a larger, system-level process for interface specification and control involving interface specifications, interface control plans and interface control documents. In this case, SCM planning for pliance with specified SCM processes and interface control takes place within the context of the system-level process.

1.4 SCM Plan After the SCM process has been implemented, some surveillance may be necessary to ensure that the SCMP provisions are properly carried out. The plan is likely to include specific SQA requirements to ensure comprocedures. The person responsible for SCM ensures that those with the assigned responsibility perform the defined SCM tasks correctly. As part of a compliance auditing [2, ann. D] [3*, c23] activity, the SQA authority might also perform this surveillance.

The results of SCM planning for a given Using integrated SCM tools with proproject are recorded in an SCMP, a "living cess control capability can make the surveildocument" that serves as a reference for the lance task easier. Some tools facilitate process SCM process. The SCMP is maintained compliance while providing flexibility for (updated and approved) as necessary during the software engineer to adapt procedures. the software life cycle. For teams to implement Other tools enforce a specific process, leaving an SCMP, they'll typically need to develop a the software engineer with less flexibility. number of more detailed, subordinate proSurveillance requirements and the level of cedures to define how specific requirements flexibility provided to the software engineer will be met during day-to-day activities (e.g., are important considerations in tool selection.

<!-- formula-not-decoded -->

## 1.5.1 SCM Measures and Measurement [3*, c9s2, c25s2-s3]

## 2. Software Configuration Identification [2, c8]

SCM measures can be designed to provide Software configuration identification identispecific information on the evolving product, fies items to be controlled, establishes idenbut they can also provide insight into how tification schemes for the items and their well the SCM process functions and iden- versions, and establishes the tools and techtify opportunities for process improvement. niques to be used in acquiring and managing SCM process measurements enable teams to controlled items. These activities provide the monitor the effectiveness of SCM activities basis for other SCM activities. on an ongoing basis. These measurements are useful in characterizing the current 2.1 Identifying Items to Be Controlled state of the process and providing a basis for [2, c8s2.2] comparison over time. Measurement anal-yes ss  S s ss n  d    snis   sintí cess changes and corresponding updates fying the software items to be controlled. This to the SCMP.

Software libraries and the various SCM tool capabilities enable teams to extract useful figuration, selecting SCIs and developing a information about SCM process characteristics (as well as project and management information). For example, information about the e 2.1.1. Software Configuration time required to accomplish various types of changes would be useful in evaluating criteria Software configuration is the functional and for determining what levels of authority are physical characteristics of hardware or softoptimal for authorizing certain changes and ware as set forth in technical documentation in estimating the resources needed to make or achieved in a product. It can be viewed as future changes. part of an overall system configuration.

involves understanding the software configuration within the context of the system constrategy for labeling software items.

Care must be taken to keep the surveillance focused on the insights that can be gained from 1 2.1.2 Software Configuration Item the measurements, not on the measurements [2, c8s2.1] [3*, c9] themselves. Software process and product measurement is further discussed in the A CI is an item or aggregation of hardware, Software Engineering Management KA.

Audits can be carried out during the software engineering process to investigate the status of specific configuration elements or to assess the SCM process implementation. In-process SCM auditing provides a more formal mechSelecting SCIs is an important process in anism for monitoring selected aspects of the which a balance must be achieved between process and may be coordinated with the providing adequate visibility for project conSQA function. (See Software Configuration trol purposes and providing a manageable Auditing.) number of controlled items.

Sot s  s    oars orts   og Sog orrs me oa  s    n   n e  ao sn t that has been established as a CI [1]. The SCM controls various items in addition to 1.5.2 In-Process Audits of SCM the code itself. Software items with potential [3*, c1s1] to become SCIs include plans, specifications and design documentation, testing materials, software tools, source and executable code, code libraries, data and data dictionaries, and documentation for installation, maintenance, operations and software use.

## 2.2 Configuration Item Identifiers and Attributes

Example baseline attributes may include the following:

<!-- formula-not-decoded -->

Status accounting activity (explained later) gathers information about CIs while they are developed. The CIs' scheme is defined in order to establish what information must be gathered and tracked for each CI. Unique identifiers and versions are tracked.

An example scheme may include the following:

| CI name              |
|----------------------|
| CI unique identifier |
| CI description       |
| CI date(s)           |
| CI type              |
| CI owner             |

The CI's unique Identifier can use significant or nonsignificant codification. An example of significant codification could be XX-YY, where XX is the iteration abbreviation (in case of using an iterative development method) and YY is the CI abbreviation.

## 2.3 Baseline Identification

[2, c8s2.5.4, c8s2.5.5, c8s2.5.6]

A software baseline is a formally approved version of a CI (regardless of media type) that is formally designated and fixed at a specific time during the CI's life cycle. The term also refers to a particular version of an agreedupon SCI. The baseline can be changed only through formal change control procedures. A baseline, with all approved changes to the baseline, represents the current approved configuration. A baseline consists of one or more related CIs.

## 2.4 Baseline Attributes

| Baseline name              |
|----------------------------|
| Baseline unique identifier |
| Baseline description       |
| Baseline date of creation  |
| Baseline CIs               |

## 2.5 Relationships Scheme Definition

[3*, c7s4]

Relationships provide the connections required to create and sustain structure. The ability to communicate intent and manage the results are significantly enhanced when effective relationships (structuring) are in place (e.g., model-based experience (MBX) platforms). Relationship information exchange and interoperability are needed to support the applicable relationship types. The status accounting activity is responsible for gathering information about relationships among CIs.

Common types of relationships can be described according to the following schemes:

Dependencies: CI-1 and CI-2 depend mutually on each other.

Example: CI-1 depends on C1-2, and vice versa, for instance a class model depends on a sequence diagram, because any change on any of both types of models, affect the other.

| CI-1 Code   | CI-2 Code   | Date   |
|-------------|-------------|--------|

Derivation: One CI derives from another, typically in a sequential relationship, not because of a lack of resources to handle both CIs but because of a constraint that requires that, for instance, CI-1 is completed before CI-2 is developed.

Example: CI-1 derives from CI-2.

| CI-1 Code   | CI-2 Code   | Date   |
|-------------|-------------|--------|

Succession: Software items evolve as a software project proceeds. A software item version is an identified instance of an item. It can be thought of as a state of an evolving item. This is what the succession relationship

[2, c8s2.5.4]

Baseline attributes are used in the status accounting activity and specify information about the baseline established.

Successions Records: According to the scheme defined for succession relationships, the next table gives the date when each CI was created (three first rows), and the fourth row indicates a change made on the CI-1 on 10/05/2021, where the current version was 1 and created new version is 2.

<!-- image -->

Dependency Record: According to the scheme defined for dependencies CI-1 and CI-2 have a dependency relationship created the day CI-2 was developed.

| CI-1   |    |   1 | 10/01/2021   |
|--------|----|-----|--------------|
| CI-2   |    |   1 | 10/04/2021   |
| CI-3   |    |   1 | 10/03/2021   |
| CI-1   |  1 |   2 | 10/05/2021   |

Derivation Record: According to the scheme defined for derivation, CI-3 derives from CI-1 and this relationship came up the day CI-3 was created.

| CI-1   | CI-2   | 10/04/2021   |
|--------|--------|--------------|

Figure 8.2. Example of reported relationships

| CI-3   | CI-1   | 10/03/2021   |
|--------|--------|--------------|

reflects, and it is reflexive in that each CI has components. Components can be source code, this relationship with itself. The first succeslibraries, modules and other artifacts; they sion comes up the first time a CI is created. can be open source or proprietary, free or Each time it is changed, a new succession paid; and the data can be widely available or comes up, and tracking these successions is access-restricted. the way to track CI versions.

Example: CI versions along a timeline.

| CI Code   | Current Version   | Next Version   | Date   |
|-----------|-------------------|----------------|--------|

Variants are program versions resulting from engineered alternative options. This type of relationship is not as common as the type of relationships described above because it is more expensive to maintain.

A simple example of the relationships among three CIs in an SBOM, called CI-1, CI-2 and CI-3, is illustrated in Figure 8.2.

## 2.6 Software Libraries

<!-- formula-not-decoded -->

A software library is a controlled collection of source code, scripts, object code, documentation and related artifacts. Requirements The decision on what relationships to track and test cases are stored in a repository and throughout the project is important because should be linked with the code baselines tracking some relationships can require extra developed. Source code is stored in a version work. On the other hand, tracking such relacontrol system, which provides traceability tionships can facilitate decisions on change and security for the baselines developed. requests (CRs) for a CI. Multiple development streams are supported Relationships between CIs can be tracked in version control systems linked to the binary in a Software Bill of Materials (SBOM). objects (e.g., object code) derived during the An SBOM is a formal record containing build process. These binary objects are typithe details and supply chain relationships cally stored in a repository that should conof the CIs used in building software. CIs tain cryptographic hashes used to perform the in an SBOM are frequently referred to as physical configuration audit (PCA).

Figure 8.3. Flow of a Change Control Process

<!-- image -->

The definitive media library contains the to those rules. The rest of this section can release baseline(s) of the artifacts that can be useful when no specific rules regarding be deployed to the test, stage and produc- change control exist in the company or the tion systems. industrial sector where the software project

The release management process depends under development is allocated. on these software libraries to manage the artifacts deployed. In terms of access control and 3.1 Requesting, Evaluating, and Approving the backup facilities, security is a key aspect of Software Changes library management. [2, c9s2.4] [3*, c11s1] [4*, c25s3]

3. Software Configuration Change Control The first step in managing changes to con[2, c9] [3*,c8] [4*, c25s3] [5, c11.s3.3] trolled items is determining what changes to make. The software change request (SCR) Software configuration change control is conprocess (Figure 8.3) provides formal procerned with changes required to CIs during cedures for submitting and recording CRs; the software life cycle. It covers the proevaluating the potential cost and impact of a cess for determining what changes to make, proposed change; and accepting, modifying, the authority for approving certain changes, deferring or rejecting the proposed change. support for implementing those changes, A CR is a request to expand or reduce the and the concept of formal deviations from project scope; modify policies, processes, project requirements as well as waivers of plans or procedures; modify costs or budgets; them. Information derived from these activ- modify implemented code; or revise schedules ities is useful in measuring change traffic and [1]. Requests for changes to SCIs may be origbreakage, as well as aspects of rework. inated by anyone at any point in the software Given that change to CIs can follow spe- life cycle and may include a suggested solution cific rules depending on the industrial sector, and requested priority. One source of a CR is area, company, etc., it is very important to the initiation of corrective action in response identify those rules in the context of the to problem reports. Regardless of the source, software project for which the SCM pro- the type of change (e.g., defect or enhancecess is being developed and to adhere strictly ment) is usually recorded on the SCR.

Recording of the SCR enables the software originating CRs, enforcing the change process engineers to track defects and collect change flow, capturing CCB decisions and reporting change process information. Linking this tool capability with the problem-reporting system can facilitate the problem resolution tracking and how quickly solutions are developed.

<!-- formula-not-decoded -->

A CR application must include the following:

- A CR form, which must describe the request and give the rationale for it
- A change certification form (necessary if the CR is granted)

These forms can be managed through the corresponding supporting tool, but humans [2, c9s2.2] [3*, c11s1] [4*, c25s3] are responsible for designing the forms.

activity measurements by change type. Once an SCR is received, a technical evaluation (also known as an impact analysis) is performed to determine the extent of the modifications necessary should the CR be accepted. A good understanding of the relationships among software (and, possibly, hardware) items is important for this task. The information recorded about the CIs' relationships could be useful for making decisions affecting any CI, given the potential impact on other CIs. Finally, an established authority — commensurate with the affected baseline, the SCI involved and the nature of the change — will evaluate the CR's technical and managerial aspects and accept, modify, reject or defer the proposed change.

<!-- formula-not-decoded -->

The authority for accepting or rejecting pro- 3.2 Implementing Software Changes posed changes rests with an entity known as a [4*, c25s3] configuration control board (CCB). In smaller projects, this authority may reside with the Approved SCRs are implemented using the leader or an assigned individual rather than defined software procedures per the applicable a multi-person board. There can be multiple schedule requirements. Because a number of levels of change authority depending on a approved SCRs might be implemented simulvariety of criteria — such as the criticality of taneously, a means for tracking which SCRs the item involved, the nature of the change are incorporated into particular software ver(e.g., impact on budget and schedule), or sions and baselines must be provided. At the where the project is in the life cycle. The comend of the change process, completed changes position of the CCBs used for a system varies may undergo configuration audits and softdepending on these criteria (but an SCM repware quality verification, which includes resentative is always present). All stakeholders ensuring that only approved changes have appropriate to the CCB level are represented. been made. The SCR process typically docuWhen a CCB's scope of authority is limited ments the change's SCM and other approval to software, the board is known as a Software information. Configuration Control Board (SCCB). The CCB's activities are subject to software quality audits or reviews.

Changes may be supported by source code version control tools. These tools allow a team of software engineers, or a single software engineer, to track and document changes to 3.1.2 Software Change Request Process the source code. These tools provide a single [3*, c1s4, c8s4] [4*, c25s3] repository for storing the source code, so they can prevent more than one software engineer An effective SCR process requires the use from editing the same module at the same

of supporting tools and procedures for time, and they record all changes made to the source code. Software engineers check mod- logical schemes defined in the activity configules out of the repository, make changes, doc- uration identification for CIs, baselines and ument the changes, and then save the edited relationships for gathering information. modules in the repository. If needed, changes can also be discarded, restoring a previous 4.1 Software Configuration Status Information baseline. More powerful tools can support [2, c10s2.1] parallel development and geographically distributed environments. These tools may manifest as separate, specialized applications under system for capturing, verifying, validating an independent SCM group's control. They and reporting necessary information as the may also appear as an integrated part of the life cycle proceeds. As in any information software engineering environment. Finally, system, the configuration status information they may be as elementary as a rudimentary to be managed for the evolving configurations change control system that is provided with an operating system.

## 3.3 Deviations and Waivers

The constraints imposed on a software engineering effort or the specifications produced other related activities. during the development activities might conThe types of information available include tain provisions that those working on the but are not limited to the following: project find cannot be satisfied at the designated point in the life cycle. A deviation is Ongoing and approved configuration a written authorization granted before the identification manufacture of an item to depart from a parCurrent implementation status of changes ticular performance or design requirement for Impacted CIs and related systems a specific number of units or a specific period Deviations and waivers of time. A waiver is a written authorization Verification and validation (V&amp;V) to allow a CI or other designated item in activities response to an issue found during production or after the project is submitted for inspection Automated tools support SCSA as tasks to depart from specified requirements when are performed, and reporting is available in a the CI or project is nevertheless considered user-friendly format. suitable for use, either as it is or after rework via an approved method. In these cases, a 4.2 Software Configuration Status Reporting formal process is used to gain approval for [2, c10s2.4] [3*, c1s5, c9s1] deviations from or waivers of the provisions.

The SCSA activity designs and operates a must be identified, collected and maintained. In addition, the information itself should be secured where relevant. SCSA information and measurements are needed to support the [1, c3] SCM process and to meet the configuration status reporting needs of management, software engineering, security, performance and

Reported information can be used by var4. Software Configuration Status ious organizational and project elements — Accounting including the development team, operations, [2, c10] [3*, c9] [5, c11s3.4] security, the maintenance team, project management, software quality activities teams SCSA is an activity of CM consisting of and others. Reporting can take many forms: recording and reporting information needed to automated reports, ad hoc queries to answer manage a configuration effectively regarding specific questions, and regular production of CIs, baselines and relationships among CIs. predesigned reports, including those develThis activity must be done by following the oped to meet security, legal or regulatory requirements. In other words, information types of formal audits might be required by produced by the SCSA activity throughout the governing contract (e.g., a contract covthe life cycle can be used to satisfy QA and ering critical software): the functional configsecurity and to provide evidence of compliuration audit (FCA) and the PCA. Successful ance with regulations, governance requirecompletion of these audits can be a prerequiments, etc. site for establishing the product baseline.

<!-- formula-not-decoded -->

In addition to reporting the configuration's current status, the information obtained by the SCSA can serve as a basis for various measurements.

Modern SCM includes a wider scope of The software FCA ensures that the audited information, including but not limited to the software item is consistent with its governing following: specifications. The software V&amp;V activities' output (see Verification and Validation in · Indicators of integrity (e.g., MAC the Software Quality KA) is a key input to this audit.

- (Message Authentication Code) SHA1 (Secure Hash Algorithm), MD5 (Message Digest))
- Indicators of security status (e.g., governance risk and compliance)

<!-- formula-not-decoded -->

Evidence of V&amp;V activities (e.g., require- The software PCA ensures that the design ments completion) and reference documentation are consistent Baseline status with the as-built software product.

- The number of CRs per SCI
- The average implement a CR

<!-- formula-not-decoded -->

## 5. Software Configuration Auditing [2, c11] [5, c11s3.5]

A software audit is an independent examination of a work product or set of work products to assess technical, security, legal and regulatory compliance with specifications, standards, contractual agreements or other This task applies to every single CI to be criteria [1]. Audits are conducted according approved as part of a baseline. The audit to a well-defined process comprising various consists of reviewing the CI to determine auditor roles and responsibilities. Because whether it satisfies requirements. How to conof this complexity, each audit must be careduct the review and the expected result must fully planned. An audit can require a number be described in the quality plan or if there is of individuals to perform various tasks over a no quality plan, defined for the software confairly short time. Tools to support the planfiguration auditing activity. ning and conduct of an audit can greatly facilitate the process.

Audits can be carried out during the development process to investigate the status of specific configuration elements. In-process audits can be applied to all baseline items to ensure that performance is consistent with specifications or that evolving documentation continues to be consistent with the developing baseline item.

Continuous reviews of CIs identified in the configuration identification activities help verify conformance to governance and regulatory requirements.

Software configuration auditing determines the extent to which an item satisfies requirements for functional and physical Configuration auditing reviews take place characteristics. Informal audits can be con- throughout project development, whenever a ducted at key points in the life cycle. Two CI must be reviewed.

6. Software Release Management and these tools vary in complexity; some require Delivery the software engineer to learn a special[2, c14] [3*, c8s2] [4*, c25s4] ized scripting language, while others use a more graphics-oriented approach that hides In this context, release refers to distrib- much of the complexity of an "intelligent"

uting software and related artifacts outside build facility. the development activity, including internal The build process and products are often releases and distribution to customers. When subject to software quality verification. different versions of a software item are avail- Outputs of the build process might be needed able for delivery (such as versions for different for future reference. They may become records platforms or versions with varying capabili- of quality, security, or compliance with orgaties), re-creating specific versions and packnizational or regulatory requirements. The aging the correct materials for version delivery SBOM listing the artifacts included in the are frequently necessary. The software library build is an important CM output. is a key element in accomplishing release and delivery tasks.

In continuous integration, software building is performed automatically when changes to CIs are committed to a source 6.1 Software Building [4*, c25s2] control repository. Tools running on a local or cloud-based server monitor the project's Software building constructs the correct versource control system and initiate a pipeline of sions of SCIs, using the appropriate configsteps to be undertaken every time a change is uration data, into a software package for committed to a particular branch or area of the delivery to a customer or other recipient such source code repository. The tool is configured as a team performing testing. For systems to retrieve a fresh copy of the complete source with hardware or firmware, the executable code for the project and execute the necessary program is delivered to the system-building commands to compile and link the code. This activity. Build instructions help ensure that configuration is often combined with steps to the proper build steps are taken in the corvalidate coding standards via automated static rect sequence. In addition to building softanalysis, execute unit tests and determine ware for new releases, SCM must usually code coverage metrics, or extract documentabe able to reproduce previous releases for tion from the source code. The resulting artirecovery, testing, maintenance or additional facts are then deployed through the Release release purposes. Management process.

## 6.2 Software Release Management

Software is built using supporting tools, such as compilers. For example, if it is necessary to rebuild an exact copy of a previously built SCI, supporting tools and associated build instructions must be under SCM control to ensure the availability of the correct versions of the tools.

[4*, c25s2]

Software release management encompasses the identification, packaging and delivery of the elements of a product (e.g., an executTool capability is useful for selecting the able program, documentation, release notes, correct versions of software items for a target or configuration data). Given that product environment and automating the process changes can occur continually, one concern ol  se   en ls   oi  n en version and configuration data. This tool to issue a release. The severity of the probcapability is necessary for projects with lems addressed by the release and measureparallel or distributed development enviments of the fault densities of prior releases ronments. Most software engineering enviaffect this decision. The packaging task idenronments provide this capability. However, tifies which product items are to be delivered and then selects the correct variants of those items, given the product's intended application. The information documenting the physical contents of a release is known as a version description document (VDD). The release notes describe new capabilities, known problems and platform requirements necessary for proper product operation. The package to be released also contains installation or upgrade instructions. The latter can be complicated because some users might have versions that are several releases old. In some cases, release management might be necessary to track the product's distribution to various customers or target systems (e.g., when the supplier was required to notify a customer of newly reported problems). Finally, a mechanism to help ensure the released item's integrity can be implemented (e.g., by including a digital signature).

- The configuration management system (CMS) provides enabling technology and logic to facilitate CM activities.
- Version control stores the source code, configuration files and related artifacts.
- Build automation (pipeline) is established to enable continuous delivery.
- A repository stores binaries that are created during the build process to extract the latest build artifacts and redeploy them as required — used in the release verification process.
- Configuration management database (CMDB) or similar persistence store.
- Change control tools.
- Release/deployment tools.

The CMS supports the unique identification of artifacts. Both individual artifacts and collections are specified in CM systems and A tool capability is needed for supporting related repositories. Structuring creates a logthese release management functions. For ical relationship between artifacts. Validation example, a connection with the tool capaand release establish the artifacts' integbility supporting the CR process is useful to rity, as part of the release management promap release contents to the SCRs that have cess. Baselines are identified where stability is been received. This tool capability might also intended. For example, interface management maintain information on various target platis identified and controlled, making it part of forms and customer environments. the baseline process. Change management, In continuous delivery, a pipeline is estabincluding variants and nonconformances, lished to build software continuously, as is reviewed and approved, and its impledescribed in the previous section. The resulting mentation is planned. Verification and audit artifacts from the build process include exeactivities are performed as part of the identicutable code and libraries, which can then be fication, change and release management procombined into an installation package and cess. Status and performance accounting are deployed into an environment for verification recorded as events occur and are made availor production use. able through the CMS.

## 7. Software Configuration Management Tools

[3*, c26s1]

Many tools can assist with CM at many levels. The scope of these tools varies depending on who uses the tools. CM is most effective when integrated with other processes and by extension with other existing tools. The selection of CM tool can be made depending on the scope that the tool is going to have.

Overview of tools:

Individual support tools are typically sufficient for small organizations or development groups that do not issue variants of their software products or face other complex SCM requirements. The following are examples of these tools:

- Version control tools: These tools track, document and store individual CIs such as source code and external documentation.
- Build handling tools: In their simplest form, such tools compile and link an executable version of the software. More

advanced building tools extract the latest environments. Such tools are appropriate for version from the version control soft- medium-to-large organizations that use variware, perform quality checks, run regres- ants of their software products and parallel sion tests, and produce various forms of development and do not have certification reports, among other tasks. requirements.

Change control tools: These tools priCompanywide-process support tools can marily support the control of CRs and automate portions of a companywide proevent notifications (e.g., CR status cess, providing support for workflow manchanges, milestones reached). agement, roles and responsibilities. They can handle many items, large volumes of data, and Project-related support tools mainly sup- numerous life cycles. In addition, such tools

port workspace management for develop- add to project-related support by supporting a ment teams and integrators. In addition, more formal development process, including they can support distributed development certification requirements.

## MATRIX OFTOPICS VS. REFERENCE MATERIAL

| Topic                                                  | Hass 2003 3    | Sommerville 2016 4   |
|--------------------------------------------------------|----------------|----------------------|
| 1. Management of the SCM Process                       |                |                      |
| 1.1. Organizational Context for SCM                    | Introduction   | c25                  |
| 1.2. Constraints and Guidance for the SCM Process      | c2,c5          |                      |
| 1.3. Planning for SCM                                  | c23            | c25                  |
| 1.3.1. SCM Organization and Responsibilities           | c10-11         | c25                  |
| 1.3.2. SCM Resources and Schedules                     | c23            |                      |
| 1.3.3. Tool Selection and Implementation               | c26s2, s6      |                      |
| 1.3.4. Vendor/Subcontractor Control                    | c13s9-c14s2    |                      |
| 1.3.5. Interface Control                               | c23s4          |                      |
| 1.4. SCM Plan                                          | c23            |                      |
| 1.5. Surveillance of Software Configuration Management | c11s3          |                      |
| 1.5.1. SCM Measures and Measurement                    | c9s2; c25s2-s3 |                      |
| 1.5.2. In-Process Audits of SCM                        | c1s1           |                      |
| 2. Software Configuration Identification               |                |                      |
| 2.1. Identifying Items to Be Controlled                | c1s2           |                      |
| 2.1.1. Software Configuration                          |                |                      |

| 2.1.2. Software Configuration Item                         | c9         |       |
|------------------------------------------------------------|------------|-------|
| 2.2. Configuration Item Identifiers and Attributes         | c9         |       |
| 2.3. Baseline Identification                               |            |       |
| 2.4. Baseline Attributes                                   |            |       |
| 2.5. Relationships Scheme Definition                       | C74        |       |
| 2.6. Software Libraries                                    | c1s3       |       |
| 3. Software Configuration Change Control                   | c8         | c25s3 |
| 3.1. Requesting, Evaluating and Approving Software Changes | c11s1      | c25s3 |
| 3.1.1. Software Configuration Control Board                | c11s1      | c25s3 |
| 3.1.2. Software Change Request Process                     | c1s4, c8s4 | c25s3 |
| 3.1.3. Software Change Request Forms Definition            | c8s4       | c25s3 |
| 3.2. Implementing Software Changes                         |            | c25s3 |
| 3.3. Deviations and Waivers                                |            |       |
| 4. Software Configuration Status Accounting                | c9         |       |
| 4.1. Software Configuration Status Information             |            |       |
| 4.2. Software Configuration Status Reporting               | c1s5; c9s1 |       |
| 5. Software Configuration Auditing                         |            |       |
| 5.1. Software Functional Configuration Audit               |            |       |
| 5.2. Software Physical Configuration Audit                 |            |       |
| 5.3. In-Process Audits of a Software Baseline              |            |       |
| 6. Software Release Management and Delivery                | c8s2       | c25s4 |
| 6.1. Software Building                                     |            | c25s2 |
| 6.2. Software Release Management                           |            | c25s2 |
| 7. Software Configuration Management Tools                 | c26s1      |       |

## FURTHER READINGS

S.P. Berczuk and B. Appleton, Software Configuration Management Patterns: Effective Teamwork, Practical Integration [6].

This book expresses useful SCM practices level 2, it suggests CM activities. and strategies as patterns. The patterns can be implemented using various tools, but they are expressed in a tool-agnostic fashion.

CMMI for Development, Version 2.0 - 2.1, pp. 66-80 [7].

This model presents a collection of best practices to help software development organizations improve their processes. At maturity

B. Aiello and L. A. Sachs, Configuration management best practices: Practical methods that work in the real world (1st edition), 2011 [8].

This book presents the seven types of change [5] J.W. Moore, The Road Map to Software control (Chapter 4, Section 3).

## REFERENCES

- [1] ISO/IEC/IEEE, "ISO/IEC/IEEE 24765:2017 Systems and Software Engineering — Vocabulary," 2nd ed. 2017.
- [2] IEEE. IEEE Standard 8282012, Standard for Configuration Management in Systems and Software Engineering, 2012.
3. [3*] A.M.J. Hass. Configuration Management Principles and Practices, 1st ed. Boston: Addison-Wesley, 2003.
4. [4*] I. Sommerville, Software Engineering, 10th ed. Global ed. Pearson, 2016.
5. Engineering: A Standards-Based Guide, 1st ed. Hoboken, NJ: Wiley-IEEE Computer Society Press, 2006.
- [6] S.P. Berczuk and B. Appleton, Software Configuration Management Patterns: Effective Teamwork, Practical Integration: Addison-Wesley Professional, 2003.
- [7] CMMI for development, Version 2.0, CMMI Institute, 2018.
8. [8]B. Aiello and L.A. Sachs, Configuration management best practices: Practical methods that work in the real world (1st edition), 2011.